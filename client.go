package maprobe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Client struct {
	mackerel     *mackerel.Client
	backupClient *backupClient
}

func newClient(ctx context.Context, apiKey string, backupStream string) *Client {
	c := &Client{
		mackerel: mackerel.NewClient(apiKey),
	}
	if backupStream != "" {
		slog.Info("setting backup firehose stream", "stream", backupStream)
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			slog.Error("failed to load AWS config", "error", err)
			return c
		}
		c.backupClient = &backupClient{
			svc:        firehose.NewFromConfig(cfg),
			streamName: backupStream,
		}
	}
	if os.Getenv("EMULATE_FAILURE") != "" {
		// force fail for POST requests
		c.mackerel.HTTPClient.Transport = &postFailureTransport{}
	}
	return c
}

func (client *Client) FindHosts(p *mackerel.FindHostsParam) ([]*mackerel.Host, error) {
	key, err := cacheKey(p)
	if err != nil {
		return nil, err
	}
	hosts, err := client.mackerel.FindHosts(p)
	if err != nil {
		if cachedHosts, found := findHostsCache.Load(key); found {
			slog.Warn("probes find host failed, using previous cache", "error", err)
			hosts = cachedHosts.([]*mackerel.Host)
		} else {
			return nil, err
		}
	} else {
		findHostsCache.Store(key, hosts)
	}
	return hosts, nil
}

func (c *Client) PostServiceMetricValues(ctx context.Context, serviceName string, mvs []*mackerel.MetricValue) error {
	err := c.mackerel.PostServiceMetricValues(serviceName, mvs)
	if err == nil {
		return nil
	}
	if c.backupClient == nil {
		return err
	}
	slog.Warn("failed to post metrics to mackerel", "error", err)
	return c.backupClient.PostServiceMetricValues(ctx, serviceName, mvs)
}

func (c *Client) PostHostMetricValues(ctx context.Context, mvs []*mackerel.HostMetricValue) error {
	err := c.mackerel.PostHostMetricValues(mvs)
	if err == nil {
		return nil
	}
	if c.backupClient == nil {
		return err
	}
	slog.Warn("failed to post metrics to mackerel", "error", err)
	return c.backupClient.PostHostMetricValues(ctx, mvs)
}

func (c *Client) fetchLatestMetricValues(hostIDs []string, metricNames []string) (mackerel.LatestMetricValues, error) {
	to := time.Now().Add(-1 * time.Minute)
	from := to.Add(metricTimeMargin)
	result := make(mackerel.LatestMetricValues, len(hostIDs))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, hostID := range hostIDs {
		for _, metricName := range metricNames {
			wg.Add(1)
			clientSem <- struct{}{}
			go func(hostID, metricName string) {
				defer func() {
					<-clientSem
					wg.Done()
				}()
				slog.Debug("fetching host metric values",
					"hostID", hostID,
					"metricName", metricName,
					"from", from.Format(time.RFC3339),
					"to", to.Format(time.RFC3339),
				)
				mvs, err := c.mackerel.FetchHostMetricValues(hostID, metricName, from.Unix(), to.Unix())
				if err != nil {
					slog.Warn("failed to fetch host metric values",
						"error", err,
						"hostID", hostID,
						"metricName", metricName,
						"from", from,
						"to", to)
					return
				}
				if len(mvs) == 0 {
					return
				}
				mu.Lock()
				if result[hostID] == nil {
					result[hostID] = make(map[string]*mackerel.MetricValue, len(metricNames))
				}
				result[hostID][metricName] = &(mvs[len(mvs)-1])
				mu.Unlock()
			}(hostID, metricName)
		}
	}
	wg.Wait()
	return result, nil
}

type postFailureTransport struct{}

func (t *postFailureTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Method == http.MethodPost || req.Method == http.MethodPut {
		return nil, fmt.Errorf("method %s FAILED FORCE", req.Method)
	}
	return http.DefaultTransport.RoundTrip(req)
}
