package core

import (
	"net/http"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
)

func TestBuildFetchClientsFingerprintsProxyEvenWhenUseIsLocal(t *testing.T) {
	fetch := jobspec.Fetch{Use: "local", Ladder: true, Profile: "chrome", Proxy: "http://p.test"}

	_, proxyClient, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if proxyClient == http.DefaultClient {
		t.Fatal("proxyClient is http.DefaultClient -- a laddered proxy rung must get its own fingerprinted client, not the bare default")
	}
}

func TestBuildFetchClientsFingerprintsProxyWhenUseIsProxyWithNoURL(t *testing.T) {
	fetch := jobspec.Fetch{Use: "proxy", Profile: "chrome"}

	_, proxyClient, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if proxyClient == http.DefaultClient {
		t.Fatal("proxyClient is http.DefaultClient -- use: proxy must be fingerprinted even with no proxy URL set")
	}
}

func TestBuildFetchClientsLeavesProxyAtDefaultWhenNotConfigured(t *testing.T) {
	fetch := jobspec.Fetch{Use: "local", Profile: "chrome"}

	_, proxyClient, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if proxyClient != http.DefaultClient {
		t.Fatal("proxyClient should stay http.DefaultClient when no proxy is configured at all")
	}
}

func TestBuildFetchClientsAlwaysFingerprintsLocal(t *testing.T) {
	fetch := jobspec.Fetch{Use: "local", Profile: "chrome"}

	localClient, _, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if localClient == http.DefaultClient {
		t.Fatal("localClient is http.DefaultClient, want a fingerprinted client")
	}
}
