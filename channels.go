package maprobe

type Channels struct {
	ServiceMetrics    chan ServiceMetric
	HostMetrics       chan HostMetric
	AggregatedMetrics chan ServiceMetric
	OtelMetrics       chan Metric
	Destination       *DestinationConfig
}

func NewChannels(dst *DestinationConfig) *Channels {
	chs := Channels{
		ServiceMetrics:    make(chan ServiceMetric, PostMetricBufferLength*10),
		HostMetrics:       make(chan HostMetric, PostMetricBufferLength*10),
		AggregatedMetrics: make(chan ServiceMetric, PostMetricBufferLength*10),
		OtelMetrics:       make(chan Metric, PostMetricBufferLength*10),
		Destination:       dst,
	}
	return &chs
}

func (ch *Channels) SendServiceMetric(m ServiceMetric) {
	if ch.Destination.Mackerel.Enabled {
		ch.ServiceMetrics <- m
	}
	if ch.Destination.Otel.Enabled {
		ch.OtelMetrics <- m.Metric
	}
}

func (ch *Channels) SendHostMetric(m HostMetric) {
	if ch.Destination.Mackerel.Enabled {
		ch.HostMetrics <- m
	}
	if ch.Destination.Otel.Enabled {
		ch.OtelMetrics <- m.Metric
	}
}

func (ch *Channels) SendAggregatedMetric(m ServiceMetric) {
	if ch.Destination.Mackerel.Enabled {
		ch.AggregatedMetrics <- m
	}
	// TODO: Otel Aggregated Metrics
}

func (ch *Channels) Close() {
	close(ch.ServiceMetrics)
	close(ch.HostMetrics)
	close(ch.AggregatedMetrics)
	close(ch.OtelMetrics)
}
