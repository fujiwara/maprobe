package maprobe

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/mackerelio/mackerel-client-go"
)

type backupClient struct {
	svc        *firehose.Client
	streamName string
}

type backupPayload struct {
	Service          string                      `json:"service,omitempty"`
	MetricValues     []*mackerel.MetricValue     `json:"metric_values,omitempty"`
	HostMetricValues []*mackerel.HostMetricValue `json:"host_metric_values,omitempty"`
}

func (c *backupClient) PostServiceMetricValues(service string, mvs []*mackerel.MetricValue) error {
	log.Printf("[info] post %d service metrics to backup stream: %s", len(mvs), c.streamName)
	data, err := json.Marshal(backupPayload{
		Service:      service,
		MetricValues: mvs,
	})
	if err != nil {
		return err
	}
	_, err = c.svc.PutRecord(context.TODO(), &firehose.PutRecordInput{
		DeliveryStreamName: &c.streamName,
		Record:             &types.Record{Data: data},
	})
	return err
}

func (c *backupClient) PostHostMetricValues(mvs []*mackerel.HostMetricValue) error {
	log.Printf("[info] post %d host metrics to backup stream: %s", len(mvs), c.streamName)
	data, err := json.Marshal(backupPayload{
		HostMetricValues: mvs,
	})
	if err != nil {
		return err
	}
	_, err = c.svc.PutRecord(context.TODO(), &firehose.PutRecordInput{
		DeliveryStreamName: &c.streamName,
		Record:             &types.Record{Data: data},
	})
	return err
}
