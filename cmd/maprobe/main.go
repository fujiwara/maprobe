package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/fujiwara/maprobe"
)

func main() {
	var config string
	flag.StringVar(&config, "config", "", "config file path")
	flag.Parse()
	err := maprobe.Run(context.Background(), config)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
