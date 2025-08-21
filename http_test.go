package maprobe_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var HTTPServerURL string
var HTTPSServerURL string

func testHTTPServer() string {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(w, "Hello HTTP Test")
	})
	ts := httptest.NewServer(handler)
	return ts.URL
}

func testHTTPSServer() string {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(w, "Hello HTTPS Test")
	})

	// Generate a test certificate that expires in 30 days
	cert, key := generateTestCertificate(30 * 24 * time.Hour)

	ts := httptest.NewUnstartedServer(handler)
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  key,
		}},
	}
	ts.StartTLS()
	return ts.URL
}

func generateTestCertificate(validFor time.Duration) (*x509.Certificate, *rsa.PrivateKey) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(validFor),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:    []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		panic(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		panic(err)
	}

	return cert, priv
}

func TestHTTP(t *testing.T) {
	pc := &maprobe.HTTPProbeConfig{
		URL:           HTTPServerURL,
		ExpectPattern: "^Hello",
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(probe, err)

	ms, err := probe.Run(context.Background())
	if err != nil {
		t.Error(err)
	}

	if len(ms) != 4 {
		t.Error("unexpected metrics num")
	}
	for _, m := range ms {
		switch m.Name {
		case "http.respose_time.seconds":
			if m.Value < 0.1 {
				t.Error("elapsed time too short")
			}
		case "http.check.ok":
			if m.Value != 1 {
				t.Error("check failed")
			}
		case "http.status.code":
			if m.Value != 200 {
				t.Errorf("unexpected status %f", m.Value)
			}
		case "http.content.length":
			if m.Value != 15 {
				t.Errorf("unexpected content length %f", m.Value)
			}
		}
	}
	t.Log(ms.String())
}

func TestHTTPS(t *testing.T) {
	pc := &maprobe.HTTPProbeConfig{
		URL:                HTTPSServerURL,
		ExpectPattern:      "^Hello",
		NoCheckCertificate: true, // Accept self-signed certificate
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(probe, err)

	ms, err := probe.Run(context.Background())
	if err != nil {
		t.Error(err)
	}

	// Should have 5 metrics: check.ok, response_time, status.code, content.length, certificate.expires_in_days
	if len(ms) != 5 {
		t.Errorf("unexpected metrics num: got %d, want 5", len(ms))
	}

	var foundCertMetric bool
	for _, m := range ms {
		switch m.Name {
		case "http.response_time.seconds":
			if m.Value < 0.1 {
				t.Error("elapsed time too short")
			}
		case "http.check.ok":
			if m.Value != 1 {
				t.Error("check failed")
			}
		case "http.status.code":
			if m.Value != 200 {
				t.Errorf("unexpected status %f", m.Value)
			}
		case "http.content.length":
			if m.Value != 16 { // "Hello HTTPS Test"
				t.Errorf("unexpected content length %f", m.Value)
			}
		case "http.certificate.expires_in_days":
			foundCertMetric = true
			// Should be around 30 days (certificate expires in 30 days)
			if m.Value < 29 || m.Value > 31 {
				t.Errorf("unexpected certificate expiration days: %f", m.Value)
			}
		}
	}

	if !foundCertMetric {
		t.Error("certificate.expires_in_days metric not found")
	}
	t.Log(ms.String())
}
