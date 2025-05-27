package maprobe

import (
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/mackerelio/mackerel-client-go"
)

type backupClient struct {
	svc        *firehose.Firehose
	streamName string
}

type backupPayload struct {
	Service          string                      `json:"service,omitempty"`
	MetricValues     []*mackerel.MetricValue     `json:"metric_values,omitempty"`
	HostMetricValues []*mackerel.HostMetricValue `json:"host_metric_values,omitempty"`
}

func (c *backupClient) PostServiceMetricValues(service string, mvs []*mackerel.MetricValue) error {
	slog.Info("post service metrics to backup stream", "count", len(mvs), "stream", c.streamName)
	data, err := json.Marshal(backupPayload{
		Service:      service,
		MetricValues: mvs,
	})
	if err != nil {
		return err
	}
	_, err = c.svc.PutRecord(&firehose.PutRecordInput{
		DeliveryStreamName: &c.streamName,
		Record:             &firehose.Record{Data: data},
	})
	return err
}

func (c *backupClient) PostHostMetricValues(mvs []*mackerel.HostMetricValue) error {
	slog.Info("post host metrics to backup stream", "count", len(mvs), "stream", c.streamName)
	data, err := json.Marshal(backupPayload{
		HostMetricValues: mvs,
	})
	if err != nil {
		return err
	}
	_, err = c.svc.PutRecord(&firehose.PutRecordInput{
		DeliveryStreamName: &c.streamName,
		Record:             &firehose.Record{Data: data},
	})
	return err
}
