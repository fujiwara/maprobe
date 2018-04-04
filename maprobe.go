package maprobe

import (
	"context"
	"log"
	"sync"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

var (
	MaxConcurrency        = 100
	PutMetricBufferLength = 100
	sem                   = make(chan struct{}, MaxConcurrency)
)

func lock() {
	sem <- struct{}{}
}

func unlock() {
	<-sem
}

func Run(ctx context.Context, configPath string) error {
	log.Println("[info] starting maprobe")
	conf, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	client := mackerel.NewClient(conf.APIKey)
	ticker := time.NewTicker(time.Minute)
	ch := make(chan Metric, PutMetricBufferLength*10)
	go putMetricWorker(ctx, client, ch)

	for {
		var wg sync.WaitGroup
	PROBE_CONFIG:
		for _, pc := range conf.ProbesConfig {
			log.Printf("[debug] finding hosts service:%s roles:%s", pc.Service, pc.Roles)
			hosts, err := client.FindHosts(&mackerel.FindHostsParam{
				Service: pc.Service,
				Roles:   pc.Roles,
			})
			if err != nil {
				log.Println("[error]", err)
				continue PROBE_CONFIG
			}
			for _, host := range hosts {
				log.Printf("[debug] proving host id:%s name:%s", host.ID, host.Name)
				wg.Add(1)
				go func(host *mackerel.Host) {
					lock()
					defer unlock()
					defer wg.Done()
					for _, probe := range pc.Probes(host) {
						log.Printf("[debug] proving host id:%s name:%s probe:%#v", host.ID, host.Name, probe)
						metrics, err := probe.Run(ctx)
						if err != nil {
							log.Println("[warn] probe failed.", err)
						}
						log.Println("[debug] probed", host.ID, host.Name+"\n", metrics.String())
						for _, m := range metrics {
							ch <- m
						}
					}
				}(host)
			}
		}
		log.Println("[debug] all probes prepared")
		wg.Wait()
		log.Println("[debug] waiting for a next tick")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
	return nil
}

func putMetricWorker(ctx context.Context, client *mackerel.Client, ch chan Metric) {
	ticker := time.NewTicker(10 * time.Second)
	mvs := make([]*mackerel.HostMetricValue, 0, PutMetricBufferLength)
	for {
		select {
		case <-ctx.Done():
		case <-ticker.C:
		case m := <-ch:
			if len(mvs) < PutMetricBufferLength {
				mvs = append(mvs, m.HostMetricValue())
				continue
			}
		}
		if len(mvs) == 0 {
			continue
		}
		log.Printf("[debug] putting %d metrics to mackerel", len(mvs))
		if err := client.PostHostMetricValues(mvs); err != nil {
			log.Println("[error] failed to put metrics to Mackerel", err)
		} else {
			log.Printf("[debug] put succeeded.")
			// success. reset buffer
			mvs = make([]*mackerel.HostMetricValue, 0, PutMetricBufferLength)
		}
	}
}
