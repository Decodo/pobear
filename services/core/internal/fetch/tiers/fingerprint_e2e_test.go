package tiers

import (
	"bufio"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func withInsecureSkipVerifyForTest(t *testing.T) {
	t.Helper()
	testHookInsecureSkipVerify = true
	t.Cleanup(func() { testHookInsecureSkipVerify = false })
}

func TestNewFingerprintedClientRoundTripsOverRealTLS(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello over spoofed tls"))
	}))
	defer target.Close()

	client, err := NewFingerprintedClient("chrome", "")
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "hello over spoofed tls" {
		t.Errorf("body = %q, want %q", body, "hello over spoofed tls")
	}
}

func TestNewFingerprintedClientNeverNegotiatesHTTP2(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto)) //nolint:gosec // r.Proto is the negotiated protocol string ("HTTP/1.1"/"HTTP/2.0"), not attacker-controlled request content
	}))
	target.EnableHTTP2 = true
	target.StartTLS()
	defer target.Close()

	for _, profile := range []string{"chrome", "firefox", "safari"} {
		t.Run(profile, func(t *testing.T) {
			client, err := NewFingerprintedClient(profile, "")
			if err != nil {
				t.Fatalf("NewFingerprintedClient: %v", err)
			}

			resp, err := client.Get(target.URL)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			defer resp.Body.Close()

			if resp.Proto != "HTTP/1.1" {
				t.Errorf("resp.Proto = %q, want %q (server supports h2, but this client must never offer it)", resp.Proto, "HTTP/1.1")
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			if string(body) != "HTTP/1.1" {
				t.Errorf("server saw protocol %q, want %q", body, "HTTP/1.1")
			}
		})
	}
}
func newForwardConnectProxy(t *testing.T, wantUser, wantPass string) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConnect(conn, wantUser, wantPass)
		}
	}()
	return ln
}

func handleConnect(conn net.Conn, wantUser, wantPass string) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	if wantUser != "" {
		auth := req.Header.Get("Proxy-Authorization")
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(wantUser+":"+wantPass))
		if auth != want {
			_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
			return
		}
	}

	target, err := net.Dial("tcp", req.Host)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer target.Close()

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(target, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(conn, target); done <- struct{}{} }()
	<-done
}

func TestNewFingerprintedClientTunnelsThroughProxyWithAuth(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("via-connect-tunnel"))
	}))
	defer target.Close()

	proxyLn := newForwardConnectProxy(t, "user", "pass")
	defer proxyLn.Close()

	proxyURL := "http://user:pass@" + proxyLn.Addr().String()
	client, err := NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("Get through proxy: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "via-connect-tunnel" {
		t.Errorf("body = %q, want %q", body, "via-connect-tunnel")
	}
}

func TestNewFingerprintedClientRejectsBadProxyAuth(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should not be reached"))
	}))
	defer target.Close()

	proxyLn := newForwardConnectProxy(t, "user", "pass")
	defer proxyLn.Close()

	proxyURL := "http://user:wrongpass@" + proxyLn.Addr().String()
	client, err := NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected an error from a rejected proxy auth, got nil")
	}
}

func TestNewFingerprintedClientForwardsPlainHTTPThroughProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain-http-via-proxy"))
	}))
	defer target.Close()

	var hit bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		_, _ = w.Write(body)
	}))
	defer proxy.Close()

	client, err := NewFingerprintedClient("chrome", proxy.URL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain-http-via-proxy" {
		t.Errorf("body = %q, want %q", body, "plain-http-via-proxy")
	}
	if !hit {
		t.Error("expected the configured proxy to receive the plain HTTP request, it never did")
	}
}

func newSOCKS5Proxy(t *testing.T, wantUser, wantPass string) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleSOCKS5(conn, wantUser, wantPass)
		}
	}()
	return ln
}

func handleSOCKS5(conn net.Conn, wantUser, wantPass string) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	greeting := make([]byte, 2)
	if _, err := io.ReadFull(r, greeting); err != nil || greeting[0] != 0x05 {
		return
	}
	methods := make([]byte, greeting[1])
	if _, err := io.ReadFull(r, methods); err != nil {
		return
	}

	wantAuth := wantUser != ""
	chosen := byte(0xFF)
	for _, m := range methods {
		if wantAuth && m == 0x02 {
			chosen = 0x02
		}
		if !wantAuth && m == 0x00 && chosen == 0xFF {
			chosen = 0x00
		}
	}
	if _, err := conn.Write([]byte{0x05, chosen}); err != nil || chosen == 0xFF {
		return
	}

	if chosen == 0x02 {
		sub := make([]byte, 2)
		if _, err := io.ReadFull(r, sub); err != nil {
			return
		}
		uname := make([]byte, sub[1])
		if _, err := io.ReadFull(r, uname); err != nil {
			return
		}
		plen := make([]byte, 1)
		if _, err := io.ReadFull(r, plen); err != nil {
			return
		}
		passwd := make([]byte, plen[0])
		if _, err := io.ReadFull(r, passwd); err != nil {
			return
		}
		ok := string(uname) == wantUser && string(passwd) == wantPass
		status := byte(0x00)
		if !ok {
			status = 0x01
		}
		if _, err := conn.Write([]byte{0x01, status}); err != nil || !ok {
			return
		}
	}

	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(r, reqHeader); err != nil || reqHeader[0] != 0x05 || reqHeader[1] != 0x01 {
		return
	}
	var host string
	switch reqHeader[3] {
	case 0x01:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(r, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	case 0x03:
		l := make([]byte, 1)
		if _, err := io.ReadFull(r, l); err != nil {
			return
		}
		domain := make([]byte, l[0])
		if _, err := io.ReadFull(r, domain); err != nil {
			return
		}
		host = string(domain)
	case 0x04:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(r, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	default:
		return
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(r, portBuf); err != nil {
		return
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])
	targetAddr := net.JoinHostPort(host, strconv.Itoa(port))

	target, err := net.Dial("tcp", targetAddr)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(target, r); done <- struct{}{} }()
	go func() { _, _ = io.Copy(conn, target); done <- struct{}{} }()
	<-done
}

func TestNewFingerprintedClientTunnelsThroughSOCKS5Proxy(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("via-socks5-tunnel"))
	}))
	defer target.Close()

	proxyLn := newSOCKS5Proxy(t, "", "")
	defer proxyLn.Close()

	proxyURL := "socks5://" + proxyLn.Addr().String()
	client, err := NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("Get through SOCKS5 proxy: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "via-socks5-tunnel" {
		t.Errorf("body = %q, want %q", body, "via-socks5-tunnel")
	}
}

func TestNewFingerprintedClientTunnelsThroughSOCKS5ProxyWithAuth(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("via-socks5-auth-tunnel"))
	}))
	defer target.Close()

	proxyLn := newSOCKS5Proxy(t, "user", "pass")
	defer proxyLn.Close()

	proxyURL := "socks5://user:pass@" + proxyLn.Addr().String()
	client, err := NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("Get through SOCKS5 proxy: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "via-socks5-auth-tunnel" {
		t.Errorf("body = %q, want %q", body, "via-socks5-auth-tunnel")
	}
}

func TestNewFingerprintedClientRejectsBadSOCKS5Auth(t *testing.T) {
	withInsecureSkipVerifyForTest(t)

	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should not be reached"))
	}))
	defer target.Close()

	proxyLn := newSOCKS5Proxy(t, "user", "pass")
	defer proxyLn.Close()

	proxyURL := "socks5://user:wrongpass@" + proxyLn.Addr().String()
	client, err := NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected an error from rejected SOCKS5 auth, got nil")
	}
}

func TestNewFingerprintedClientForwardsPlainHTTPThroughSOCKS5Proxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain-http-via-socks5"))
	}))
	defer target.Close()

	proxyLn := newSOCKS5Proxy(t, "", "")
	defer proxyLn.Close()

	proxyURL := "socks5://" + proxyLn.Addr().String()
	client, err := NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain-http-via-socks5" {
		t.Errorf("body = %q, want %q", body, "plain-http-via-socks5")
	}
}
