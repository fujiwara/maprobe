package maprobe

import "time"

type CommandProbe struct {
	Command string
	Timeout time.Duration
}
