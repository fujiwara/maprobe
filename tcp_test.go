package maprobe_test

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var TCPServerAddress net.Addr

func TestMain(m *testing.M) {
	TCPServerAddress = testTCPServer()
	HTTPServerURL = testHTTPServer()
	ret := m.Run()
	os.Exit(ret)
}

func testTCPServer() net.Addr {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Fatal(err)
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
			if m.Value != 0 {
				t.Error("check failed")
			}
		}
	}
	t.Log(ms.String())
}
