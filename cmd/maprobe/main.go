package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/alecthomas/kong"
	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/fujiwara/maprobe"
	gops "github.com/google/gops/agent"
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

	var kongCtx *kong.Context
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// detect running on AWS Lambda
		slog.Info("running on AWS Lambda")
		// Override os.Args for Lambda
		originalArgs := os.Args
		os.Args = []string{os.Args[0], "lambda"}
		kongCtx = kong.Parse(&cli)
		os.Args = originalArgs
	} else {
		kongCtx = kong.Parse(&cli)
	}

	// Setup structured logging
	setupSlog(cli.LogLevel, cli.LogFormat)
	
	slog.Info("maprobe", "version", maprobe.Version)

	if cli.GopsEnabled {
		if err := gops.Listen(gops.Options{}); err != nil {
			slog.Error("failed to start gops agent", "error", err)
			os.Exit(1)
		}
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, trapSignals...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fullCommandName := kongCtx.Command()
	// Extract the base command name (Kong may return "command <arg>" format)
	cmdName, _, _ := strings.Cut(fullCommandName, " ")
	sigCount := 0
	go func() {
		for sig := range sigCh {
			if cmdName == "agent" {
				slog.Info("signal received, shutting down", "signal", sig)
			} else {
				slog.Debug("signal received, shutting down", "signal", sig)
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
	var err error
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
		slog.Info("running on AWS Lambda", "config", cli.Lambda.Config)
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
		err = fmt.Errorf("command %s does not exist", cmdName)
	}
	wg.Wait()
	slog.Info("shutdown")
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
		slog.Debug("finding host", "id", id)
		client := mackerel.NewClient(apikey)
		return client.FindHost(id)
	}
	slog.Debug("using dummy host")
	return &mackerel.Host{ID: "dummy"}, nil
}

func runProbe(ctx context.Context, id string, pc maprobe.ProbeConfig) error {
	slog.Debug("probe config", "config", fmt.Sprintf("%#v", pc))
	host, err := mackerelHost(id)
	if err != nil {
		return err
	}
	slog.Debug("host", "host", marshalJSON(host))
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

func setupSlog(logLevel, logFormat string) {
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "trace", "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
		AddSource: true,
	}
	
	switch strings.ToLower(logFormat) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	
	slog.SetDefault(slog.New(handler))
}
