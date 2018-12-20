package maprobe

import (
	"log"
	"sync"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

func fetchLatestMetricValues(client *mackerel.Client, hostIDs []string, metricNames []string) (mackerel.LatestMetricValues, error) {
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
				log.Printf("[trace] fetching host metric values: %s %s from %s to %s", hostID, metricName, from, to)
				mvs, err := client.FetchHostMetricValues(hostID, metricName, from.Unix(), to.Unix())
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
