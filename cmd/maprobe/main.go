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

	"github.com/alecthomas/kong"
	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/fujiwara/maprobe"
	gops "github.com/google/gops/agent"
	"github.com/hashicorp/logutils"
	mackerel "github.com/mackerelio/mackerel-client-go"
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
)

func main() {
	var cli maprobe.CLI

	// Parse command line arguments
	var args []string
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// detect running on AWS Lambda
		log.Println("[info] running on AWS Lambda")
		args = []string{"lambda"}
	} else {
		args = os.Args[1:]
	}

	parser, err := kong.New(&cli)
	if err != nil {
		log.Println("[error]", err)
		os.Exit(1)
	}
	
	kongCtx, err := parser.Parse(args)
	if err != nil {
		log.Println("[error]", err)
		os.Exit(1)
	}

	log.Println("[info] maprobe", maprobe.Version)

	if cli.GopsEnabled {
		if err := gops.Listen(gops.Options{}); err != nil {
			log.Fatal(err)
		}
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(cli.LogLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, trapSignals...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdName := kongCtx.Command()
	sigCount := 0
	go func() {
		for sig := range sigCh {
			if cmdName == "agent" {
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
	switch cmdName {
	case "version":
		fmt.Printf("maprobe version %s\n", maprobe.Version)
		return
	case "agent":
		if cli.Agent.WithFirehoseEndpoint {
			wg.Add(1)
			go maprobe.RunFirehoseEndpoint(ctx, &wg, cli.Agent.Port)
		}
		wg.Add(1)
		err = maprobe.Run(ctx, &wg, cli.Agent.Config, false)
	case "once":
		wg.Add(1)
		err = maprobe.Run(ctx, &wg, cli.Once.Config, true)
	case "lambda":
		log.Println("[info] Running on AWS Lambda with config", cli.Lambda.Config)
		golambda.StartWithOptions(func(lambdaCtx context.Context) error {
			wg.Add(1)
			return maprobe.Run(lambdaCtx, &wg, cli.Lambda.Config, true)
		}, golambda.WithContext(ctx))
	case "ping":
		err = runProbe(ctx, cli.Ping.HostID, &maprobe.PingProbeConfig{
			Address: cli.Ping.Address,
			Count:   cli.Ping.Count,
			Timeout: cli.Ping.Timeout,
		})
	case "tcp":
		err = runProbe(ctx, cli.TCP.HostID, &maprobe.TCPProbeConfig{
			Host:               cli.TCP.Host,
			Port:               cli.TCP.Port,
			Timeout:            cli.TCP.Timeout,
			Send:               cli.TCP.Send,
			Quit:               cli.TCP.Quit,
			ExpectPattern:      cli.TCP.ExpectPattern,
			NoCheckCertificate: cli.TCP.NoCheckCertificate,
			TLS:                cli.TCP.TLS,
		})
	case "http":
		err = runProbe(ctx, cli.HTTP.HostID, &maprobe.HTTPProbeConfig{
			URL:                cli.HTTP.URL,
			Method:             cli.HTTP.Method,
			Body:               cli.HTTP.Body,
			Headers:            cli.HTTP.Headers,
			Timeout:            cli.HTTP.Timeout,
			ExpectPattern:      cli.HTTP.ExpectPattern,
			NoCheckCertificate: cli.HTTP.NoCheckCertificate,
		})
	case "firehose-endpoint":
		wg.Add(1)
		maprobe.RunFirehoseEndpoint(ctx, &wg, cli.FirehoseEndpoint.Port)
	default:
		err = fmt.Errorf("command %s not exist", cmdName)
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


func marshalJSON(i interface{}) string {
	b, _ := json.Marshal(i)
	return string(b)
}
