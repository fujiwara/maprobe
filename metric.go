package maprobe

import "time"

type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
}
