package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/fujiwara/maprobe"
	gops "github.com/google/gops/agent"
	"github.com/hashicorp/logutils"
	mackerel "github.com/mackerelio/mackerel-client-go"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func init() {
	maprobe.MackerelAPIKey = os.Getenv("MACKEREL_APIKEY")
}

var (
	trapSignals = []os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}

	app         = kingpin.New("maprobe", "")
	logLevel    = app.Flag("log-level", "log level").Default("info").OverrideDefaultFromEnvar("LOG_LEVEL").String()
	gopsEnabled = app.Flag("gops", "enable gops agent").Default("false").OverrideDefaultFromEnvar("GOPS").Bool()

	version = app.Command("version", "Show version")

	agent                     = app.Command("agent", "Run agent")
	agentConfig               = agent.Flag("config", "configuration file path or URL(http|s3)").Short('c').OverrideDefaultFromEnvar("CONFIG").String()
	agentWithFirehoseEndpoint = agent.Flag("with-firehose-endpoint", "run with firehose HTTP endpoint server").Bool()
	agentPort                 = agent.Flag("port", "firehose HTTP endpoint listen port").Default("8080").Int()

	once       = app.Command("once", "Run once")
	onceConfig = once.Flag("config", "configuration file path or URL(http|s3)").Short('c').OverrideDefaultFromEnvar("CONFIG").String()

	lambda       = app.Command("lambda", "Run on AWS Lambda like once mode")
	lambdaConfig = lambda.Flag("config", "configuration file path or URL(http|s3)").Short('c').OverrideDefaultFromEnvar("CONFIG").String()

	ping        = app.Command("ping", "Run ping probe")
	pingAddress = ping.Arg("address", "Hostname or IP address").Required().String()
	pingCount   = ping.Flag("count", "Iteration count").Short('c').Int()
	pingTimeout = ping.Flag("timeout", "Timeout to ping response").Short('t').Duration()
	pingHostID  = ping.Flag("host-id", "Mackerel host ID").Short('i').String()

	tcp                   = app.Command("tcp", "Run TCP probe")
	tcpHost               = tcp.Arg("host", "Hostname or IP address").Required().String()
	tcpPort               = tcp.Arg("port", "Port number").Required().String()
	tcpSend               = tcp.Flag("send", "String to send to the server").Short('s').String()
	tcpQuit               = tcp.Flag("quit", "String to send server to initiate a clean close of the connection").Short('q').String()
	tcpTimeout            = tcp.Flag("timeout", "Timeout").Short('t').Duration()
	tcpExpectPattern      = tcp.Flag("expect", "Regexp pattern to expect in server response").Short('e').String()
	tcpNoCheckCertificate = tcp.Flag("no-check-certificate", "Do not check certificate").Short('k').Bool()
	tcpHostID             = tcp.Flag("host-id", "Mackerel host ID").Short('i').String()

	http                   = app.Command("http", "Run HTTP probe")
	httpURL                = http.Arg("url", "URL").Required().String()
	httpMethod             = http.Flag("method", "Request method").Default("GET").Short('m').String()
	httpBody               = http.Flag("body", "Request body").Short('b').String()
	httpExpectPattern      = http.Flag("expect", "Regexp pattern to expect in server response").Short('e').String()
	httpTimeout            = http.Flag("timeout", "Timeout").Short('t').Duration()
	httpNoCheckCertificate = http.Flag("no-check-certificate", "Do not check certificate").Short('k').Bool()
	httpHeaders            = HTTPHeader(http.Flag("header", "Request headers").Short('H').PlaceHolder("Header: Value"))
	httpHostID             = http.Flag("host-id", "Mackerel host ID").Short('i').String()

	firehoseEndpoint     = app.Command("firehose-endpoint", "Run Firehose HTTP endpoint")
	firehoseEndpointPort = firehoseEndpoint.Flag("port", "Listen port").Default("8080").Short('p').Int()
)

func main() {
	log.Println("[info] maprobe", maprobe.Version)

	if *gopsEnabled {
		if err := gops.Listen(gops.Options{}); err != nil {
			log.Fatal(err)
		}
	}

	var args []string
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// detect running on AWS Lambda
		log.Println("[info] running on AWS Lambda")
		args = []string{"lambda"}
	} else {
		args = os.Args[1:]
	}
	sub, err := app.Parse(args)
	if err != nil {
		log.Println("[error]", err)
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

	var wg sync.WaitGroup
	switch sub {
	case "version":
		fmt.Printf("maprobe version %s\n", maprobe.Version)
		return
	case "agent":
		if *agentWithFirehoseEndpoint {
			wg.Add(1)
			go maprobe.RunFirehoseEndpoint(ctx, &wg, *agentPort)
		}
		wg.Add(1)
		err = maprobe.Run(ctx, &wg, *agentConfig, false)
	case "once":
		wg.Add(1)
		err = maprobe.Run(ctx, &wg, *onceConfig, true)
	case "lambda":
		log.Println("[info] Running on AWS Lambda with config", *lambdaConfig)
		golambda.StartWithOptions(func(ctx context.Context) error {
			wg.Add(1)
			return maprobe.Run(ctx, &wg, *lambdaConfig, true)
		}, golambda.WithContext(ctx))
	case "ping":
		err = runProbe(ctx, *pingHostID, &maprobe.PingProbeConfig{
			Address: *pingAddress,
			Count:   *pingCount,
			Timeout: *pingTimeout,
		})
	case "tcp":
		err = runProbe(ctx, *tcpHostID, &maprobe.TCPProbeConfig{
			Host:               *tcpHost,
			Port:               *tcpPort,
			Timeout:            *tcpTimeout,
			Send:               *tcpSend,
			Quit:               *tcpQuit,
			ExpectPattern:      *tcpExpectPattern,
			NoCheckCertificate: *tcpNoCheckCertificate,
		})
	case "http":
		err = runProbe(ctx, *httpHostID, &maprobe.HTTPProbeConfig{
			URL:                *httpURL,
			Method:             *httpMethod,
			Body:               *httpBody,
			Headers:            httpHeaders.Value,
			Timeout:            *httpTimeout,
			ExpectPattern:      *httpExpectPattern,
			NoCheckCertificate: *httpNoCheckCertificate,
		})
	case "firehose-endpoint":
		wg.Add(1)
		maprobe.RunFirehoseEndpoint(ctx, &wg, *firehoseEndpointPort)
	default:
		err = fmt.Errorf("command %s not exist", sub)
	}
	wg.Wait()
	log.Println("[info] shutdown")
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

func mackerelHost(id string) (*mackerel.Host, error) {
	if apikey := os.Getenv("MACKEREL_APIKEY"); id != "" && apikey != "" {
		log.Printf("[debug] finding host id:%s", id)
		client := mackerel.NewClient(apikey)
		return client.FindHost(id)
	}
	log.Printf("[debug] using dummy host")
	return &mackerel.Host{ID: "dummy"}, nil
}

func runProbe(ctx context.Context, id string, pc maprobe.ProbeConfig) error {
	log.Printf("[debug] %#v", pc)
	host, err := mackerelHost(id)
	if err != nil {
		return err
	}
	log.Printf("[debug] host: %s", marshalJSON(host))
	p, err := pc.GenerateProbe(host)
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

func marshalJSON(i interface{}) string {
	b, _ := json.Marshal(i)
	return string(b)
}
