package maprobe

type Metric interface {
	ServiceMetric | HostMetric
}
