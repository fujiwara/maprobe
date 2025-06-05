package maprobe_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var TCPServerAddress net.Addr
var TLSServerAddress net.Addr

func TestMain(m *testing.M) {
	TCPServerAddress = testTCPServer()
	TLSServerAddress = testTLSServer()
	HTTPServerURL = testHTTPServer()
	HTTPSServerURL = testHTTPSServer()
	ret := m.Run()
	os.Exit(ret)
}

func testTCPServer() net.Addr {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				time.Sleep(100 * time.Millisecond)
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					io.WriteString(conn, scanner.Text()+"\n")
				}
			}(conn)
		}
	}()
	return l.Addr()
}

func testTLSServer() net.Addr {
	// Generate a test certificate that expires in 30 days
	cert, key := generateTestCertificate(30 * 24 * time.Hour)
	
	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
	}
	
	config := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
	
	l, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		panic(err)
	}
	
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				time.Sleep(100 * time.Millisecond)
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					io.WriteString(conn, scanner.Text()+"\n")
				}
			}(conn)
		}
	}()
	return l.Addr()
}


func TestTCP(t *testing.T) {
	host, port, _ := net.SplitHostPort(TCPServerAddress.String())

	pc := &maprobe.TCPProbeConfig{
		Host:          "{{.Host.Name}}",
		Port:          port,
		Send:          "hello\n",
		ExpectPattern: "^hello",
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test", Name: host})
	if err != nil {
		t.Error(err)
	}
	ms, err := probe.Run(context.Background())
	if err != nil {
		t.Error(err)
	}
	if len(ms) != 2 {
		t.Error("unexpected metrics num")
	}
	for _, m := range ms {
		switch m.Name {
		case "tcp.elapsed.seconds":
			if m.Value < 0.1 {
				t.Error("elapsed time too short")
			}
		case "tcp.check.ok":
			if m.Value != 1 {
				t.Error("check failed")
			}
		}
	}
	t.Log(ms.String())
}

func TestTCPFail(t *testing.T) {
	host, port, _ := net.SplitHostPort(TCPServerAddress.String())

	pc := &maprobe.TCPProbeConfig{
		Host:          "{{.Host.Name}}",
		Port:          port,
		Send:          "hello\n",
		ExpectPattern: "^world",
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test", Name: host})
	if err != nil {
		t.Error(err)
	}
	ms, err := probe.Run(context.Background())
	if err == nil {
		t.Error("expected error, but got nil")
		return
	}
	if len(ms) != 2 {
		t.Error("unexpected metrics num")
	}
	for _, m := range ms {
		switch m.Name {
		case "tcp.elapsed.seconds":
			if m.Value < 0.1 {
				t.Error("elapsed time too short")
			}
		case "tcp.check.ok":
			if m.Value != 0 {
				t.Error("check failed")
			}
		}
	}
	t.Log(ms.String())
}

func TestTCPTLS(t *testing.T) {
	host, port, _ := net.SplitHostPort(TLSServerAddress.String())

	pc := &maprobe.TCPProbeConfig{
		Host:               "{{.Host.Name}}",
		Port:               port,
		Send:               "hello\n",
		ExpectPattern:      "^hello",
		TLS:                true,
		NoCheckCertificate: true, // Accept self-signed certificate
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test", Name: host})
	if err != nil {
		t.Error(err)
	}
	ms, err := probe.Run(context.Background())
	if err != nil {
		t.Error(err)
	}
	
	// Should have 3 metrics: check.ok, elapsed.seconds, certificate.expires_in_days
	if len(ms) != 3 {
		t.Errorf("unexpected metrics num: got %d, want 3", len(ms))
	}
	
	var foundCertMetric bool
	for _, m := range ms {
		switch m.Name {
		case "tcp.elapsed.seconds":
			if m.Value < 0.1 {
				t.Error("elapsed time too short")
			}
		case "tcp.check.ok":
			if m.Value != 1 {
				t.Error("check failed")
			}
		case "tcp.certificate.expires_in_days":
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
