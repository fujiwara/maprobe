package maprove

import (
	"regexp"
	"time"
)

type HTTPProve struct {
	URL                string
	Method             string
	Headers            map[string]string
	Body               string
	ExpectPattern      *regexp.Regexp
	InsecureSkipVerify bool
	Timeout            time.Duration
}
