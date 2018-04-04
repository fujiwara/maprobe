package maprobe

import (
	"regexp"
	"time"
)

type HTTPProbe struct {
	URL                string
	Method             string
	Headers            map[string]string
	Body               string
	ExpectPattern      *regexp.Regexp
	InsecureSkipVerify bool
	Timeout            time.Duration
}
