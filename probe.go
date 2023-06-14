package maprobe

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Probe interface {
	Run(ctx context.Context) (Metrics, error)
	HostID() string
	MetricName(string) string
}

func newMetric[T Probe](p T, name string, value float64) Metric {
	return Metric{
		Name:      p.MetricName(name),
		Value:     value,
		Timestamp: time.Now(),
	}
}

type ProbeConfig interface {
	GenerateProbe(host *mackerel.Host) (Probe, error)
}

var (
	expandCache = sync.Map{}
)

func newFuncMap(env map[string]string) template.FuncMap {
	return template.FuncMap{
		"env": func(keys ...string) string {
			v := ""
			for _, k := range keys {
				if v, ok := env[k]; ok {
					return v
				}
				if v := os.Getenv(k); v != "" {
					return v
				}
				v = k
			}
			return v
		},
		"must_env": func(key string) string {
			if v, ok := env[key]; ok {
				return v
			} else if v, ok := os.LookupEnv(key); ok {
				return v
			}
			panic(fmt.Sprintf("environment variable %s is not defined", key))
		},
	}
}

func expandCacheKey(src string, env map[string]string) string {
	if env == nil {
		return src
	}
	return fmt.Sprintf("%s%s", env, src)
}

func expandPlaceHolder(src string, host *mackerel.Host, env map[string]string) (string, error) {
	var err error

	if !strings.Contains(src, "{{") {
		// no need to expand
		return src, nil
	}

	var tmpl *template.Template
	key := expandCacheKey(src, env)
	if _tmpl, ok := expandCache.Load(key); ok {
		log.Println("[trace] expand cache HIT", key)
		tmpl = _tmpl.(*template.Template)
	} else {
		log.Println("[trace] expand cache MISS", key)
		tmpl, err = template.New(key).Funcs(newFuncMap(env)).Parse(src)
		if err != nil {
			return "", err
		}
		expandCache.Store(src, tmpl)
	}
	var b strings.Builder
	b.Grow(len(src))
	err = tmpl.Execute(&b, templateParam{Host: host})
	return b.String(), err
}
