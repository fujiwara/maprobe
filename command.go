package maprove

import "time"

type CommandProve struct {
	Command string
	Timeout time.Duration
}
