package tiers

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

func TestHelloIDForProfile(t *testing.T) {
	cases := []struct {
		profile string
		want    utls.ClientHelloID
	}{
		{"chrome", utls.HelloChrome_Auto},
		{"firefox", utls.HelloFirefox_Auto},
		{"safari", utls.HelloSafari_Auto},
	}
	for _, c := range cases {
		got, err := helloIDForProfile(c.profile)
		if err != nil {
			t.Errorf("helloIDForProfile(%q) returned error: %v", c.profile, err)
		}
		if got != c.want {
			t.Errorf("helloIDForProfile(%q) = %+v, want %+v", c.profile, got, c.want)
		}
	}
}

func TestHelloIDForProfileRejectsUnknown(t *testing.T) {
	if _, err := helloIDForProfile("edge"); err == nil {
		t.Error("expected an error for an unrecognized profile, got nil")
	}
	if _, err := helloIDForProfile(""); err == nil {
		t.Error("expected an error for an empty profile, got nil")
	}
}

func captureFirstTLSRecord(t *testing.T, ln net.Listener) <-chan []byte {
	t.Helper()
	ch := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		header := make([]byte, 5)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		length := int(header[3])<<8 | int(header[4])
		body := make([]byte, length)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		ch <- append(header, body...)
	}()
	return ch
}

func captureDialFingerprintedHello(t *testing.T, helloID utls.ClientHelloID) []byte {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ch := captureFirstTLSRecord(t, ln)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dialFingerprinted(ctx, "tcp", ln.Addr().String(), helloID, nil)
	if conn != nil {
		_ = conn.Close()
	}
	_ = err

	select {
	case b := <-ch:
		return b
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting to capture a ClientHello")
		return nil
	}
}

func captureStockClientHello(t *testing.T) []byte {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ch := captureFirstTLSRecord(t, ln)

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: "example.com", InsecureSkipVerify: true}) //nolint:gosec // test-only; nothing ever replies to complete this handshake
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = tlsConn.HandshakeContext(ctx)
	_ = tlsConn.Close()

	select {
	case b := <-ch:
		return b
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting to capture the stock ClientHello")
		return nil
	}
}

func TestDialFingerprintedSendsADistinctClientHelloPerProfile(t *testing.T) {
	chromeID, _ := helloIDForProfile("chrome")
	firefoxID, _ := helloIDForProfile("firefox")
	safariID, _ := helloIDForProfile("safari")

	chromeHello := captureDialFingerprintedHello(t, chromeID)
	firefoxHello := captureDialFingerprintedHello(t, firefoxID)
	safariHello := captureDialFingerprintedHello(t, safariID)
	stockHello := captureStockClientHello(t)

	if bytes.Equal(chromeHello, firefoxHello) {
		t.Error("chrome and firefox produced identical ClientHello bytes")
	}
	if bytes.Equal(chromeHello, safariHello) {
		t.Error("chrome and safari produced identical ClientHello bytes")
	}
	if bytes.Equal(firefoxHello, safariHello) {
		t.Error("firefox and safari produced identical ClientHello bytes")
	}

	for profile, hello := range map[string][]byte{"chrome": chromeHello, "firefox": firefoxHello, "safari": safariHello} {
		if bytes.Equal(hello, stockHello) {
			t.Errorf("profile %q produced the same ClientHello as stock crypto/tls -- fingerprinting had no effect", profile)
		}
	}
}
