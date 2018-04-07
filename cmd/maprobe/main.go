package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/fujiwara/maprobe"
	"github.com/hashicorp/logutils"
)

func main() {
	var config, logLevel string
	flag.StringVar(&config, "config", "", "config file path")
	flag.StringVar(&logLevel, "log-level", "info", "log-level")
	flag.Parse()

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	err := maprobe.Run(context.Background(), config)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
