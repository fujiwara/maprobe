package maprobe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
)

var (
	DefaultHTTPTimeout         = 15 * time.Second
	DefaultHTTPMetricKeyPrefix = "http"
)

type HTTPProbeConfig struct {
	URL                string            `yaml:"url"`
	Method             string            `yaml:"method"`
	Headers            map[string]string `yaml:"header"`
	Body               string            `yaml:"body"`
	ExpectPattern      string            `yaml:"expect_pattern"`
	Timeout            time.Duration     `yaml:"timeout"`
	NoCheckCertificate bool              `yaml:"no_check_certificate"`
	MetricKeyPrefix    string            `yaml:"metric_key_prefix"`
}

func (pc *HTTPProbeConfig) GenerateProbe(host *mackerel.Host) (Probe, error) {
	p := &HTTPProbe{
		hostID:             host.ID,
		metricKeyPrefix:    pc.MetricKeyPrefix,
		Timeout:            pc.Timeout,
		NoCheckCertificate: pc.NoCheckCertificate,
	}
	var err error
	p.URL, err = expandPlaceHolder(pc.URL, host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid URL")
	}
	if !strings.HasPrefix(p.URL, "http://") && !strings.HasPrefix(p.URL, "https://") {
		return nil, errors.New("invalid URL " + p.URL)
	}

	p.Headers = make(map[string]string, len(pc.Headers))
	for name, value := range pc.Headers {
		p.Headers[name], err = expandPlaceHolder(value, host)
		if err != nil {
			return nil, errors.Wrap(err, "invalid header "+name)
		}
	}

	p.Body, err = expandPlaceHolder(pc.Body, host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid body")
	}

	var pattern string
	pattern, err = expandPlaceHolder(pc.ExpectPattern, host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid expect_pattern")
	}
	if pattern != "" {
		p.ExpectPattern, err = regexp.Compile(pattern)
		if err != nil {
			return nil, errors.Wrap(err, "invalid expect_pattern")
		}
	}

	// default
	if p.Method == "" {
		p.Method = http.MethodGet
	}
	if p.Timeout == 0 {
		p.Timeout = DefaultHTTPTimeout
	}
	if p.metricKeyPrefix == "" {
		p.metricKeyPrefix = DefaultHTTPMetricKeyPrefix
	}

	return p, nil
}

type HTTPProbe struct {
	hostID          string
	metricKeyPrefix string

	URL                string
	Method             string
	Headers            map[string]string
	Body               string
	ExpectPattern      *regexp.Regexp
	Timeout            time.Duration
	NoCheckCertificate bool
}

func (p *HTTPProbe) HostID() string {
	return p.hostID
}

func (p *HTTPProbe) MetricName(name string) string {
	return p.metricKeyPrefix + "." + name
}

func (p *HTTPProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *HTTPProbe) Run(ctx context.Context) (ms Metrics, err error) {
	var ok bool
	start := time.Now()
	defer func() {
		elapsed := time.Now().Sub(start)
		ms = append(ms, newMetric(p, "response_time.seconds", elapsed.Seconds()))
		if ok {
			ms = append(ms, newMetric(p, "check.ok", 1))
		} else {
			ms = append(ms, newMetric(p, "check.ok", 0))
		}
		log.Println("[trace]", ms.String())
	}()

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	req, err := http.NewRequest(p.Method, p.URL, strings.NewReader(p.Body))
	if err != nil {
		log.Println("[warn] invalid HTTP request", err)
		return
	}
	for name, value := range p.Headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("Connection", "close") // do not keep alive to health check.

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.NoCheckCertificate},
	}
	client := &http.Client{Transport: tr}

	log.Printf("[debug] http request %s %s", req.Method, req.URL)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("[warn] HTTP request failed", err)
		return
	}
	defer resp.Body.Close()

	ms = append(ms, newMetric(p, "status.code", float64(resp.StatusCode)))
	if resp.StatusCode >= 400 {
		ok = false
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("[warn] HTTP read body failed", err)
		return ms, errors.Wrap(err, "read body failed")
	}
	ms = append(ms, newMetric(p, "content.length", float64(len(body))))

	if p.ExpectPattern != nil {
		if !p.ExpectPattern.Match(body) {
			return ms, errors.Wrap(err, "unexpected response")
		}
	}

	ok = true
	return
}
