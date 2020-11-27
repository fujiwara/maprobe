package maprobe

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Client struct {
	mackerel     *mackerel.Client
	backupClient *backupClient
}

func newClient(apiKey string, backupStream string) *Client {
	c := &Client{
		mackerel: mackerel.NewClient(apiKey),
	}
	if backupStream != "" {
		log.Println("[info] setting backup firehose stream:", backupStream)
		sess := session.Must(session.NewSession())
		c.backupClient = &backupClient{
			svc:        firehose.New(sess),
			streamName: backupStream,
		}
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
			log.Println("[warn] probes find host failed, using previous cache:", err)
			hosts = cachedHosts.([]*mackerel.Host)
		} else {
			return nil, err
		}
	} else {
		findHostsCache.Store(key, hosts)
	}
	return hosts, nil
}

func (c *Client) PostServiceMetricValues(serviceName string, mvs []*mackerel.MetricValue) error {
	err := c.mackerel.PostServiceMetricValues(serviceName, mvs)
	if err == nil {
		return nil
	}
	if c.backupClient == nil {
		return err
	}
	log.Println("[warn] failed to post metrics to mackerel:", err)
	return c.backupClient.PostServiceMetricValues(serviceName, mvs)
}

func (c *Client) PostHostMetricValues(mvs []*mackerel.HostMetricValue) error {
	err := c.mackerel.PostHostMetricValues(mvs)
	if err == nil {
		return nil
	}
	if c.backupClient == nil {
		return err
	}
	log.Println("[warn] failed to post metrics to mackerel:", err)
	return c.backupClient.PostHostMetricValues(mvs)
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
				log.Printf(
					"[trace] fetching host metric values: %s %s from %s to %s",
					hostID,
					metricName,
					from.Format(time.RFC3339),
					to.Format(time.RFC3339),
				)
				mvs, err := c.mackerel.FetchHostMetricValues(hostID, metricName, from.Unix(), to.Unix())
				if err != nil {
					log.Printf("[warn] failed to fetch host metric values: %s %s %s from %s to %s", err, hostID, metricName, from, to)
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
