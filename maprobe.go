package maprobe

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"os"
	"sync"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

var (
	Version                = "0.5.4"
	MaxConcurrency         = 100
	MaxClientConcurrency   = 5
	PostMetricBufferLength = 100
	sem                    = make(chan struct{}, MaxConcurrency)
	clientSem              = make(chan struct{}, MaxClientConcurrency)
	ProbeInterval          = 60 * time.Second
	mackerelRetryInterval  = 10 * time.Second
	metricTimeMargin       = -3 * time.Minute
	MackerelAPIKey         string
)

func lock() {
	sem <- struct{}{}
	log.Printf("[trace] locked. concurrency: %d", len(sem))
}

func unlock() {
	<-sem
	log.Printf("[trace] unlocked. concurrency: %d", len(sem))
}

func Run(ctx context.Context, wg *sync.WaitGroup, configPath string, once bool) error {
	defer wg.Done()
	defer log.Println("[info] stopping maprobe")

	log.Println("[info] starting maprobe")
	conf, confDigest, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	log.Println("[debug]", conf.String())
	client := newClient(MackerelAPIKey, conf.Backup.FirehoseStreamName)
	if os.Getenv("EMULATE_FAILURE") != "" {
		// force fail for POST requests
		client.mackerel.HTTPClient.Transport = &postFailureTransport{}
	}

	hch := make(chan HostMetric, PostMetricBufferLength*10)
	defer close(hch)
	sch := make(chan ServiceMetric, PostMetricBufferLength*10)
	defer close(sch)

	if len(conf.Probes) > 0 {
		wg.Add(1)
		if conf.PostProbedMetrics {
			go postHostMetricWorker(wg, client, hch)
		} else {
			go dumpHostMetricWorker(wg, hch)
		}
	}

	if len(conf.Aggregates) > 0 {
		wg.Add(1)
		if conf.PostAggregatedMetrics {
			go postServiceMetricWorker(wg, client, sch)
		} else {
			go dumpServiceMetricWorker(wg, sch)
		}
	}

	ticker := time.NewTicker(ProbeInterval)
	for {
		var wg2 sync.WaitGroup
		for _, pd := range conf.Probes {
			wg2.Add(1)
			go runProbes(ctx, pd, client, hch, &wg2)
		}
		for _, ag := range conf.Aggregates {
			wg2.Add(1)
			go runAggregates(ctx, ag, client, sch, &wg2)
		}
		wg2.Wait()
		if once {
			return nil
		}

		log.Println("[debug] waiting for a next tick")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		log.Println("[debug] checking a new config")
		newConf, digest, err := LoadConfig(configPath)
		if err != nil {
			log.Println("[warn]", err)
			log.Println("[warn] still using current config")
		} else if confDigest != digest {
			conf = newConf
			confDigest = digest
			log.Println("[info] config reloaded")
			log.Println("[debug]", conf)
		}
	}
}

func runProbes(ctx context.Context, pd *ProbeDefinition, client *Client, ch chan HostMetric, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf(
		"[debug] probes finding hosts service:%s roles:%s statuses:%v",
		pd.Service,
		pd.Roles,
		pd.Statuses,
	)
	roles := exStrings(pd.Roles)
	statuses := exStrings(pd.Statuses)

	hosts, err := client.FindHosts(&mackerel.FindHostsParam{
		Service:  pd.Service.String(),
		Roles:    roles,
		Statuses: statuses,
	})
	if err != nil {
		log.Println("[error] probes find host failed", err)
		return
	}
	log.Printf("[debug] probes %d hosts found", len(hosts))
	if len(hosts) == 0 {
		return
	}

	spawnInterval := time.Duration(int64(ProbeInterval) / int64(len(hosts)) / 2)
	if spawnInterval > time.Second {
		spawnInterval = time.Second
	}

	var wg2 sync.WaitGroup
	for _, host := range hosts {
		time.Sleep(spawnInterval)
		log.Printf("[debug] probes preparing host id:%s name:%s", host.ID, host.Name)
		wg2.Add(1)
		go func(host *mackerel.Host) {
			lock()
			defer unlock()
			defer wg2.Done()
			for _, probe := range pd.GenerateProbes(host, client.mackerel) {
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
}

func runAggregates(ctx context.Context, ag *AggregateDefinition, client *Client, ch chan ServiceMetric, wg *sync.WaitGroup) {
	defer wg.Done()

	service := ag.Service.String()
	roles := exStrings(ag.Roles)
	statuses := exStrings(ag.Statuses)
	log.Printf(
		"[debug] aggregates finding hosts service:%s roles:%s statuses:%v",
		service,
		roles,
		statuses,
	)

	hosts, err := client.FindHosts(&mackerel.FindHostsParam{
		Service:  service,
		Roles:    roles,
		Statuses: statuses,
	})
	if err != nil {
		log.Println("[error] aggregates find hosts failed", err)
		return
	}
	log.Printf("[debug] aggregates %d hosts found", len(hosts))

	hostIDs := make([]string, 0, len(hosts))
	for _, h := range hosts {
		hostIDs = append(hostIDs, h.ID)
	}
	metricNames := make([]string, 0, len(ag.Metrics))
	for _, m := range ag.Metrics {
		metricNames = append(metricNames, m.Name.String())
	}

	log.Printf("[debug] fetching latest metrics hosts:%v metrics:%v", hostIDs, metricNames)

	// TODO: If latest API will returns metrics refreshed at on minute,
	// We will replace to client.FetchLatestMetricValues().
	latest, err := client.fetchLatestMetricValues(hostIDs, metricNames)
	if err != nil {
		log.Printf("[error] fetch latest metrics failed. %s hosts:%v metrics:%v", err, hostIDs, metricNames)
		return
	}

	now := time.Now()
	for _, mc := range ag.Metrics {
		name := mc.Name.String()
		var timestamp int64
		values := []float64{}
		for hostID, metrics := range latest {
			if _v, ok := metrics[name]; ok {
				if _v == nil {
					log.Printf("[trace] latest %s:%s is not found", hostID, name)
					continue
				}
				v, ok := _v.Value.(float64)
				if !ok {
					log.Printf("[warn] latest %s:%s = %v is not a float64 value", hostID, name, _v)
					continue
				}
				ts := time.Unix(_v.Time, 0)
				log.Printf("[trace] latest %s:%s:%d = %f", hostID, name, _v.Time, v)
				if ts.After(now.Add(metricTimeMargin)) {
					values = append(values, v)
					if _v.Time > timestamp {
						timestamp = _v.Time
					}
				} else {
					log.Printf("[warn] latest %s:%s at %s is outdated", hostID, name, ts)
				}
			}
		}
		if len(hosts) > 0 && len(values) == 0 {
			log.Printf("[warn] %s:%s latest values are not found", ag.Service, mc.Name)
		}

		for _, output := range mc.Outputs {
			var value float64
			if len(values) == 0 {
				if !output.EmitZero {
					continue
				}
				timestamp = now.Add(-1 * time.Minute).Unix()
			} else {
				value = output.calc(values)
			}
			log.Printf("[debug] aggregates %s(%s)=%f -> %s:%s timestamp %d",
				output.Func, name, value,
				ag.Service, output.Name,
				timestamp,
			)
			ch <- ServiceMetric{
				Service:   ag.Service.String(),
				Name:      output.Name.String(),
				Value:     value,
				Timestamp: time.Unix(timestamp, 0),
			}
		}
	}
}

func postHostMetricWorker(wg *sync.WaitGroup, client *Client, ch chan HostMetric) {
	log.Println("[info] starting postHostMetricWorker")
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	mvs := make([]*mackerel.HostMetricValue, 0, PostMetricBufferLength)
	run := true
	for run {
		select {
		case m, cont := <-ch:
			if cont {
				mvs = append(mvs, m.HostMetricValue())
				if len(mvs) < PostMetricBufferLength {
					continue
				}
			} else {
				log.Println("[info] shutting down postHostMetricWorker")
				run = false
			}
		case <-ticker.C:
		}
		if len(mvs) == 0 {
			continue
		}
		log.Printf("[debug] posting %d host metrics to Mackerel", len(mvs))
		b, _ := json.Marshal(mvs)
		log.Println("[debug]", string(b))
		if err := client.PostHostMetricValues(mvs); err != nil {
			log.Println("[error] failed to post host metrics to Mackerel", err)
			time.Sleep(mackerelRetryInterval)
			continue
		}
		log.Printf("[debug] post host metrics succeeded.")
		// success. reset buffer
		mvs = mvs[:0]
	}
}

func postServiceMetricWorker(wg *sync.WaitGroup, client *Client, ch chan ServiceMetric) {
	log.Println("[info] starting postServiceMetricWorker")
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	mvsMap := make(map[string][]*mackerel.MetricValue)
	run := true
	for run {
		select {
		case m, cont := <-ch:
			if cont {
				if math.IsNaN(m.Value) {
					log.Printf(
						"[warn] %s:%s value NaN is not supported by Mackerel",
						m.Service, m.Name,
					)
					continue
				} else {
					mvsMap[m.Service] = append(mvsMap[m.Service], m.MetricValue())
				}
				if len(mvsMap[m.Service]) < PostMetricBufferLength {
					continue
				}
			} else {
				log.Println("[info] shutting down postServiceMetricWorker")
				run = false
			}
		case <-ticker.C:
		}

		for serviceName, mvs := range mvsMap {
			if len(mvs) == 0 {
				continue
			}
			log.Printf("[debug] posting %d service metrics to Mackerel:%s", len(mvs), serviceName)
			b, _ := json.Marshal(mvs)
			log.Println("[debug]", string(b))
			if err := client.PostServiceMetricValues(serviceName, mvs); err != nil {
				log.Printf("[error] failed to post service metrics to Mackerel:%s %s", serviceName, err)
				time.Sleep(mackerelRetryInterval)
				continue
			}
			log.Printf("[debug] post service succeeded.")
			// success. reset buffer
			mvs = mvs[:0]
			mvsMap[serviceName] = mvs
		}
	}
}

func dumpHostMetricWorker(wg *sync.WaitGroup, ch chan HostMetric) {
	defer wg.Done()
	log.Println("[info] starting dumpHostMetricWorker")
	for m := range ch {
		b, _ := json.Marshal(m.HostMetricValue())
		log.Printf("[info] %s %s", m.HostID, b)
	}
}

func dumpServiceMetricWorker(wg *sync.WaitGroup, ch chan ServiceMetric) {
	defer wg.Done()
	log.Println("[info] starting dumpServiceMetricWorker")
	for m := range ch {
		b, _ := json.Marshal(m.MetricValue())
		log.Printf("[info] %s %s", m.Service, b)
	}
}

type templateParam struct {
	Host *mackerel.Host
}
