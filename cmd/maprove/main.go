package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/fujiwara/maprove"
)

func main() {
	var config string
	flag.StringVar(&config, "config", "", "config file path")
	flag.Parse()
	err := maprove.Run(context.Background(), config)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
