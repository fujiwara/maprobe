package maprobe

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

var (
	MaxConcurrency         = 100
	PostMetricBufferLength = 100
	sem                    = make(chan struct{}, MaxConcurrency)
	ProbeInterval          = 60 * time.Second
	mackerelRetryInterval  = 10 * time.Second
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
	log.Println("[debug]", conf)
	client := mackerel.NewClient(conf.APIKey)
	ch := make(chan Metric, PostMetricBufferLength*10)

	if conf.ProbeOnly {
		go dumpMetricWorker(ctx, ch)
	} else {
		go postMetricWorker(ctx, client, ch)
	}

	var wg sync.WaitGroup
	for _, pc := range conf.ProbesConfig {
		wg.Add(1)
		go runProbeConfig(ctx, pc, client, ch, &wg)
	}
	wg.Wait()

	return nil
}

func runProbeConfig(ctx context.Context, pc *ProbeConfig, client *mackerel.Client, ch chan Metric, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(ProbeInterval)
	for {
		log.Printf("[debug] finding hosts service:%s roles:%s", pc.Service, pc.Roles)
		hosts, err := client.FindHosts(&mackerel.FindHostsParam{
			Service: pc.Service,
			Roles:   pc.Roles,
		})
		if err != nil {
			log.Println("[error]", err)
			time.Sleep(mackerelRetryInterval)
			continue
		}
		log.Printf("[debug] %d hosts found", len(hosts))
		if len(hosts) == 0 {
			time.Sleep(mackerelRetryInterval)
			continue
		}
		var wg2 sync.WaitGroup
		for _, host := range hosts {
			log.Printf("[debug] preparing host id:%s name:%s", host.ID, host.Name)
			wg2.Add(1)
			go func(host *mackerel.Host) {
				lock()
				defer unlock()
				defer wg2.Done()
				for _, probe := range pc.GenerateProbes(host) {
					log.Printf("[debug] probing host id:%s name:%s probe:%s", host.ID, host.Name, probe)
					metrics, err := probe.Run(ctx)
					if err != nil {
						log.Printf("[warn] probe failed. %s host id:%s name:%s probe:%s", err, host.ID, host.Name, probe)
					}
					for _, m := range metrics {
						ch <- m
					}
				}
			}(host)
		}
		wg2.Wait()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func postMetricWorker(ctx context.Context, client *mackerel.Client, ch chan Metric) {
	ticker := time.NewTicker(10 * time.Second)
	mvs := make([]*mackerel.HostMetricValue, 0, PostMetricBufferLength)
	for {
		select {
		case <-ctx.Done():
		case <-ticker.C:
		case m := <-ch:
			mvs = append(mvs, m.HostMetricValue())
			if len(mvs) < PostMetricBufferLength {
				continue
			}
		}
		if len(mvs) == 0 {
			continue
		}
		log.Printf("[debug] posting %d metrics to Mackerel", len(mvs))
		if err := client.PostHostMetricValues(mvs); err != nil {
			log.Println("[error] failed to post metrics to Mackerel", err)
			time.Sleep(mackerelRetryInterval)
			continue
		}
		log.Printf("[debug] post succeeded.")
		// success. reset buffer
		mvs = mvs[:0]
	}
}

func dumpMetricWorker(ctx context.Context, ch chan Metric) {
	for {
		select {
		case <-ctx.Done():
		case m := <-ch:
			b, _ := json.Marshal(m.HostMetricValue())
			log.Println("[debug]", string(b))
		}
	}
}
