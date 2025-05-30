package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alecthomas/kong"
	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/fujiwara/maprobe"
	gops "github.com/google/gops/agent"
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
	// Detect AWS Lambda environment and modify args accordingly
	args := os.Args
	if strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_Lambda") || os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		slog.Info("running on AWS Lambda")
		args = []string{args[0], "lambda"}
	}

	// Parse CLI to get log settings
	var cli maprobe.CLI
	parser, err := kong.New(&cli)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create parser: %v\n", err)
		os.Exit(1)
	}

	kongCtx, err := parser.Parse(args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse arguments: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging
	maprobe.SetupLogger(cli.LogLevel, cli.LogFormat)

	slog.Info("maprobe", "version", maprobe.Version)

	if cli.GopsEnabled {
		if err := gops.Listen(gops.Options{}); err != nil {
			slog.Error("failed to start gops agent", "error", err)
			os.Exit(1)
		}
	}

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, trapSignals...)

	fullCommandName := kongCtx.Command()
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

	// Handle Lambda execution differently
	if cmdName == "lambda" {
		slog.Info("running on AWS Lambda", "config", cli.Lambda.Config)
		golambda.StartWithOptions(func(lambdaCtx context.Context) error {
			return maprobe.Main(lambdaCtx, []string{"maprobe", "lambda", "--config", cli.Lambda.Config})
		}, golambda.WithContext(ctx))
		return
	}

	// Execute command
	err = maprobe.Main(ctx, args)

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
