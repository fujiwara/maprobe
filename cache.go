package maprobe

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/mackerelio/mackerel-client-go"
)

var findHostsCache sync.Map

func cacheKey(v interface{}) (string, error) {
	key, err := json.Marshal(v)
	return string(key), err
}

func findHosts(client *mackerel.Client, p *mackerel.FindHostsParam) ([]*mackerel.Host, error) {
	key, err := cacheKey(p)
	if err != nil {
		return nil, err
	}
	hosts, err := client.FindHosts(p)
	if err != nil {
		if cachedHosts, found := findHostsCache.Load(key); found {
			log.Println("[warn] probes find host failed, using cache", err)
			hosts = cachedHosts.([]*mackerel.Host)
		} else {
			return nil, err
		}
	} else {
		findHostsCache.Store(key, hosts)
	}
	return hosts, nil
}
