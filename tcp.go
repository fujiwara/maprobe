package maprobe

import (
	"regexp"
	"time"
)

type TCPProbe struct {
	Address       string
	Port          int
	Send          string
	ExpectPattern *regexp.Regexp
	Timeout       time.Duration
}
