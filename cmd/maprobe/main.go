package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fujiwara/maprobe"
	"github.com/hashicorp/logutils"
	mackerel "github.com/mackerelio/mackerel-client-go"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	trapSignals = []os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}

	app      = kingpin.New("maprobe", "")
	logLevel = app.Flag("log-level", "log level").Default("info").String()

	agent       = app.Command("agent", "Run agent")
	agentConfig = agent.Flag("config", "configuration file").Short('c').String()

	ping        = app.Command("ping", "Run ping probe")
	pingAddress = ping.Arg("address", "Hostname or IP address").Required().String()
	pingCount   = ping.Flag("count", "Iteration count").Short('c').Int()
	pingTimeout = ping.Flag("timeout", "Timeout to ping response").Short('t').Duration()

	tcp                   = app.Command("tcp", "Run TCP probe")
	tcpHost               = tcp.Arg("host", "Hostname or IP address").Required().String()
	tcpPort               = tcp.Arg("port", "Port number").Required().String()
	tcpSend               = tcp.Flag("send", "String to send to the server").Short('s').String()
	tcpQuit               = tcp.Flag("quit", "String to send server to initiate a clean close of the connection").Short('q').String()
	tcpTimeout            = tcp.Flag("timeout", "Timeout").Short('t').Duration()
	tcpExpectPattern      = tcp.Flag("expect", "Regexp pattern to expect in server response").Short('e').String()
	tcpNoCheckCertificate = tcp.Flag("no-check-certificate", "Do not check certificate").Short('k').Bool()

	http                   = app.Command("http", "Run HTTP probe")
	httpURL                = http.Arg("url", "URL").Required().String()
	httpMethod             = http.Flag("method", "Request method").Default("GET").Short('m').String()
	httpBody               = http.Flag("body", "Request body").Short('b').String()
	httpExpectPattern      = http.Flag("expect", "Regexp pattern to expect in server response").Short('e').String()
	httpTimeout            = http.Flag("timeout", "Timeout").Short('t').Duration()
	httpNoCheckCertificate = http.Flag("no-check-certificate", "Do not check certificate").Short('k').Bool()
	httpHeaders            = HTTPHeader(http.Flag("header", "Request headers").Short('H').PlaceHolder("Header: Value"))
)

func main() {
	var err error

	sub, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, trapSignals...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCount := 0
	go func() {
		for sig := range sigCh {
			if sub == "agent" {
				log.Println("[info] SIGNAL", sig, "shutting down")
			} else {
				log.Println("[debug] SIGNAL", sig, "shutting down")
			}
			sigCount++
			if sigCount >= 2 {
				// bailout
				os.Exit(2)
			}
			cancel()
		}
	}()

	switch sub {
	case "agent":
		err = maprobe.Run(ctx, *agentConfig)
	case "ping":
		err = runProbe(ctx, &maprobe.PingProbeConfig{
			Address: *pingAddress,
			Count:   *pingCount,
			Timeout: *pingTimeout,
		})
	case "tcp":
		err = runProbe(ctx, &maprobe.TCPProbeConfig{
			Host:               *tcpHost,
			Port:               *tcpPort,
			Timeout:            *tcpTimeout,
			Send:               *tcpSend,
			Quit:               *tcpQuit,
			ExpectPattern:      *tcpExpectPattern,
			NoCheckCertificate: *tcpNoCheckCertificate,
		})
	case "http":
		err = runProbe(ctx, &maprobe.HTTPProbeConfig{
			URL:                *httpURL,
			Method:             *httpMethod,
			Body:               *httpBody,
			Headers:            httpHeaders.Value,
			Timeout:            *httpTimeout,
			ExpectPattern:      *httpExpectPattern,
			NoCheckCertificate: *httpNoCheckCertificate,
		})
	default:
		err = fmt.Errorf("command %s not exist", sub)
	}
	select {
	case <-ctx.Done():
		return
	default:
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runProbe(ctx context.Context, pc maprobe.ProbeConfig) error {
	log.Printf("[debug] %#v", pc)
	p, err := pc.GenerateProbe(&mackerel.Host{ID: "dummy"})
	if err != nil {
		return err
	}
	ms, err := p.Run(ctx)
	if len(ms) > 0 {
		fmt.Print(ms.String())
	}
	if err != nil {
		return err
	}
	return nil
}

type httpHeader struct {
	Value map[string]string
}

func (h *httpHeader) IsCumulative() bool {
	return true
}

func (h *httpHeader) Set(value string) error {
	pairs := strings.SplitN(value, ":", 2)
	if len(pairs) != 2 {
		return fmt.Errorf("expected 'Header:Value' got '%s'", value)
	}
	name, value := pairs[0], strings.TrimLeft(pairs[1], " ")
	h.Value[name] = value
	return nil
}

func (h *httpHeader) String() string {
	return fmt.Sprintf("%v", h.Value)
}

func HTTPHeader(s kingpin.Settings) (target *httpHeader) {
	target = &httpHeader{
		Value: make(map[string]string),
	}
	s.SetValue((*httpHeader)(target))
	return
}
