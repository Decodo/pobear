package tiers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/pkg/provider"
)

type ManifestLookup interface {
	Lookup(name string) (*provider.Manifest, bool)
}

type CredentialLoader interface {
	Load(path, providerName, credential string) (secret string, ok bool, err error)
}

type Provider struct {
	Client          *http.Client
	Manifests       ManifestLookup
	Credentials     CredentialLoader
	CredentialsPath string
}

func (p *Provider) Fetch(ctx context.Context, cfg jobspec.Fetch, targetURL string) (*Result, fault.Kind, error) {
	manifest, err := p.lookupManifest(cfg.Name)
	if err != nil {
		return nil, fault.Provider, err
	}

	secret, err := p.loadCredential(manifest)
	if err != nil {
		return nil, fault.Auth, err
	}

	rt, err := resolveProviderTemplate(manifest, cfg.Template)
	if err != nil {
		return nil, fault.Provider, err
	}

	options := provider.ApplyOptionDefaults(rt.Options, cfg.Options)
	body, query, err := provider.Render(rt.Request, targetURL, options)
	if err != nil {
		return nil, fault.Provider, err
	}

	req, kind, err := buildHTTPRequest(ctx, manifest, rt.Endpoint, rt.Method, cfg, body, query, secret)
	if err != nil {
		return nil, kind, err
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fault.Timeout, redactSecret(err, secret)
		}
		return nil, fault.Network, redactSecret(err, secret)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fault.Network, err
	}

	status, extractedBody, finalURL, bodyMatched, err := provider.Extract(manifest.Response, resp.StatusCode, respBody, targetURL)
	if err != nil {
		return nil, fault.Provider, err
	}

	result := &Result{Status: status, Body: extractedBody, FinalURL: finalURL}

	if respKind := classifyProviderStatus(status, manifest.Errors); respKind != "" {
		if snippet := bodySnippet(respBody); snippet != "" {
			return result, respKind, fmt.Errorf("tiers: provider status %d: %s", status, snippet)
		}
		return result, respKind, fmt.Errorf("tiers: provider status %d", status)
	}

	if !bodyMatched {
		if snippet := bodySnippet(respBody); snippet != "" {
			return result, fault.Provider, fmt.Errorf("tiers: provider response.body %q matched nothing: %s", manifest.Response.Body, snippet)
		}
		return result, fault.Provider, fmt.Errorf("tiers: provider response.body %q matched nothing", manifest.Response.Body)
	}
	return result, "", nil
}

func redactSecret(err error, secret string) error {
	if err == nil || secret == "" {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), secret, "[REDACTED]")
	if msg == err.Error() {
		return err
	}
	return errors.New(msg)
}

func (p *Provider) lookupManifest(name string) (*provider.Manifest, error) {
	if p.Manifests != nil {
		if m, ok := p.Manifests.Lookup(name); ok {
			return m, nil
		}
	}
	return nil, fmt.Errorf("tiers: unknown provider %q", name)
}

func (p *Provider) loadCredential(manifest *provider.Manifest) (string, error) {
	secret, ok, err := p.Credentials.Load(p.CredentialsPath, manifest.Name, manifest.Auth.Credential)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("tiers: missing credential for provider %q", manifest.Name)
	}
	return secret, nil
}

func buildHTTPRequest(ctx context.Context, manifest *provider.Manifest, endpoint, method string, cfg jobspec.Fetch, body map[string]any, query map[string]string, secret string) (*http.Request, fault.Kind, error) {
	endpoint, err := applyQuery(endpoint, query)
	if err != nil {
		return nil, fault.Provider, err
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fault.Provider, err
		}
		bodyReader = bytes.NewReader(encoded)
	}

	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, fault.Network, err
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	applyHeaders(req, manifest.Headers)
	applyHeaders(req, cfg.Headers)
	if err := applyAuth(req, manifest.Auth, secret); err != nil {
		return nil, fault.Auth, err
	}
	return req, "", nil
}

func applyQuery(endpoint string, query map[string]string) (string, error) {
	if len(query) == 0 {
		return endpoint, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func applyAuth(req *http.Request, auth provider.Auth, secret string) error {
	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+secret)
	case "basic":
		req.Header.Set("Authorization", "Basic "+secret)
	case "header":
		req.Header.Set(auth.Name, secret)
	case "query":
		q := req.URL.Query()
		q.Set(auth.Name, secret)
		req.URL.RawQuery = q.Encode()
	default:
		return fmt.Errorf("tiers: unknown auth type %q", auth.Type)
	}
	return nil
}

func resolveProviderTemplate(m *provider.Manifest, templateName string) (rt *provider.ResolvedTemplate, err error) {
	rt, err = provider.ResolveTemplate(m, templateName)
	if err != nil {
		return nil, wrapTemplateSelectionError(err)
	}
	return rt, nil
}

func wrapTemplateSelectionError(err error) error {
	switch e := err.(type) {
	case *provider.ErrTemplateNotFound:
		return fmt.Errorf("tiers: %s has no template %q", e.Manifest, e.Name)
	case *provider.ErrTemplateAmbiguous:
		return fmt.Errorf("tiers: %s has more than one template and none is marked default; fetch.template is required", e.Manifest)
	default:
		return err
	}
}

func classifyProviderStatus(status int, mappings []provider.ErrorMapping) fault.Kind {
	for _, m := range mappings {
		if m.Status == status {
			return fault.Kind(m.Kind)
		}
	}
	return classifyDefaultStatus(status)
}
