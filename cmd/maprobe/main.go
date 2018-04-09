package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fujiwara/maprobe"
	"github.com/hashicorp/logutils"
)

var trapSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT,
}

func main() {
	var config, logLevel string
	flag.StringVar(&config, "config", "", "config file path")
	flag.StringVar(&logLevel, "log-level", "info", "log-level")
	flag.Parse()

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, trapSignals...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		sig := <-sigCh
		log.Println("[info] SIGNAL", sig, "shutting down")
		cancel()
	}()

	if err := maprobe.Run(ctx, config); err != nil {
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("[error]", err)
			os.Exit(1)
		}
	}
}
