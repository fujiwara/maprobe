package maprobe

type OtelMetric interface {
	ServiceMetric | HostMetric
}
