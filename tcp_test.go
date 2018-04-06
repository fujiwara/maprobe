package maprobe_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

func testTCPServer(t *testing.T) net.Addr {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer l.Close()
		for {
			conn, err := l.Accept()
			if err != nil {
				t.Fatal(err)
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
	addr := testTCPServer(t)
	host, port, _ := net.SplitHostPort(addr.String())

	pc := &maprobe.TCPProbeConfig{
		Host:          host,
		Port:          port,
		Send:          "hello\n",
		ExpectPattern: "^hello",
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test"})
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
