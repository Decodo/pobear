package tiers

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
)

var testHookInsecureSkipVerify = false

func helloIDForProfile(profile string) (utls.ClientHelloID, error) {
	switch profile {
	case "chrome":
		return utls.HelloChrome_Auto, nil
	case "firefox":
		return utls.HelloFirefox_Auto, nil
	case "safari":
		return utls.HelloSafari_Auto, nil
	default:
		return utls.ClientHelloID{}, fmt.Errorf("tiers: no TLS fingerprint for profile %q", profile)
	}
}

func NewFingerprintedClient(profile, proxyRawURL string) (*http.Client, error) {
	helloID, err := helloIDForProfile(profile)
	if err != nil {
		return nil, err
	}

	var proxyURL *url.URL
	if proxyRawURL != "" {
		proxyURL, err = url.Parse(proxyRawURL)
		if err != nil {
			return nil, fmt.Errorf("tiers: invalid proxy URL: %w", err)
		}
		switch proxyURL.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return nil, fmt.Errorf("tiers: proxy scheme %q not supported (only http/https/socks5)", proxyURL.Scheme)
		}
	}

	transport := &http.Transport{
		ForceAttemptHTTP2: false,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFingerprinted(ctx, network, addr, helloID, proxyURL)
		},
	}
	if proxyURL != nil {
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if req.URL.Scheme == "https" {
				return nil, nil
			}
			return proxyURL, nil
		}
	}
	return &http.Client{Transport: transport}, nil
}

func dialFingerprinted(ctx context.Context, network, addr string, helloID utls.ClientHelloID, proxyURL *url.URL) (net.Conn, error) {
	conn, err := dialThroughProxy(ctx, network, addr, proxyURL)
	if err != nil {
		return nil, err
	}

	sni, _, err := net.SplitHostPort(addr)
	if err != nil {
		sni = addr
	}

	cfg := &utls.Config{ServerName: sni, InsecureSkipVerify: testHookInsecureSkipVerify} //nolint:gosec // testHookInsecureSkipVerify is always false outside this package's own tests
	uconn := utls.UClient(conn, cfg, helloID)
	if err := forceALPNToHTTP11(uconn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := uconn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("tiers: TLS handshake failed: %w", err)
	}
	return uconn, nil
}

func dialThroughProxy(ctx context.Context, network, addr string, proxyURL *url.URL) (net.Conn, error) {
	if proxyURL == nil {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}

	if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
		return dialSOCKS5(ctx, network, addr, proxyURL)
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, network, proxyURL.Host)
	if err != nil {
		return nil, err
	}
	if err := connectProxyTunnel(conn, addr, proxyURL); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func dialSOCKS5(ctx context.Context, network, addr string, proxyURL *url.URL) (net.Conn, error) {
	var auth *proxy.Auth
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		auth = &proxy.Auth{User: proxyURL.User.Username(), Password: password}
	}

	dialer, err := proxy.SOCKS5(network, proxyURL.Host, auth, &net.Dialer{})
	if err != nil {
		return nil, fmt.Errorf("tiers: building SOCKS5 dialer: %w", err)
	}
	ctxDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("tiers: SOCKS5 dialer does not support context")
	}

	conn, err := ctxDialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("tiers: SOCKS5 tunnel to %s failed: %w", addr, err)
	}
	return conn, nil
}

func forceALPNToHTTP11(uconn *utls.UConn) error {
	if err := uconn.BuildHandshakeState(); err != nil {
		return fmt.Errorf("tiers: building TLS handshake state: %w", err)
	}
	for _, ext := range uconn.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = []string{"http/1.1"}
		}
	}
	return nil
}

func connectProxyTunnel(conn net.Conn, target string, proxyURL *url.URL) error {
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: make(http.Header),
	}
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		creds := base64.StdEncoding.EncodeToString([]byte(proxyURL.User.Username() + ":" + password))
		req.Header.Set("Proxy-Authorization", "Basic "+creds)
	}
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("tiers: writing CONNECT request: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return fmt.Errorf("tiers: reading CONNECT response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tiers: proxy CONNECT failed: %s", resp.Status)
	}
	if br.Buffered() > 0 {
		return fmt.Errorf("tiers: proxy sent %d unexpected bytes after CONNECT response", br.Buffered())
	}
	return nil
}
