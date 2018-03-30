package maprove

import (
	"regexp"
	"time"
)

type TCPProve struct {
	Address       string
	Port          int
	Send          string
	ExpectPattern *regexp.Regexp
	Timeout       time.Duration
}
