package maprobe

type Channels struct {
	ServiceMetrics    chan ServiceMetric
	HostMetrics       chan HostMetric
	AggregatedMetrics chan ServiceMetric
	OtelMetrics       chan Metric
}

func NewChannels() Channels {
	return Channels{
		ServiceMetrics:    make(chan ServiceMetric, PostMetricBufferLength*10),
		HostMetrics:       make(chan HostMetric, PostMetricBufferLength*10),
		AggregatedMetrics: make(chan ServiceMetric, PostMetricBufferLength*10),
		OtelMetrics:       make(chan Metric, PostMetricBufferLength*10),
	}
}

func (ch Channels) Close() {
	close(ch.ServiceMetrics)
	close(ch.HostMetrics)
	close(ch.AggregatedMetrics)
	close(ch.OtelMetrics)
}
