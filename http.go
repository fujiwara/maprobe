package maprobe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"fmt"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

var (
	DefaultHTTPTimeout         = 15 * time.Second
	DefaultHTTPMetricKeyPrefix = "http"
)

type HTTPProbeConfig struct {
	URL                string            `yaml:"url"`
	Method             string            `yaml:"method"`
	Headers            map[string]string `yaml:"headers"`
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
	p.URL, err = expandPlaceHolder(pc.URL, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !strings.HasPrefix(p.URL, "http://") && !strings.HasPrefix(p.URL, "https://") {
		return nil, fmt.Errorf("invalid URL %s", p.URL)
	}

	p.Headers = make(map[string]string, len(pc.Headers))
	for name, value := range pc.Headers {
		p.Headers[name], err = expandPlaceHolder(value, host, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid header %s: %w", name, err)
		}
	}

	p.Body, err = expandPlaceHolder(pc.Body, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid body: %w", err)
	}

	var pattern string
	pattern, err = expandPlaceHolder(pc.ExpectPattern, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid expect_pattern: %w", err)
	}
	if pattern != "" {
		p.ExpectPattern, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid expect_pattern: %w", err)
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
		elapsed := time.Since(start)
		ms = append(ms, newMetric(p, "response_time.seconds", elapsed.Seconds()))
		if ok {
			ms = append(ms, newMetric(p, "check.ok", 1))
		} else {
			ms = append(ms, newMetric(p, "check.ok", 0))
		}
		slog.Debug("http probe completed", "metrics", ms.String())
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, p.Method, p.URL, strings.NewReader(p.Body))
	if err != nil {
		slog.Warn("invalid HTTP request", "error", err)
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

	slog.Debug("http request", "method", req.Method, "url", req.URL)
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("HTTP request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	ms = append(ms, newMetric(p, "status.code", float64(resp.StatusCode)))
	if resp.StatusCode >= 400 {
		ok = false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("HTTP read body failed", "error", err)
		return ms, fmt.Errorf("read body failed: %w", err)
	}
	ms = append(ms, newMetric(p, "content.length", float64(len(body))))

	if p.ExpectPattern != nil {
		if !p.ExpectPattern.Match(body) {
			return ms, fmt.Errorf("unexpected response")
		}
	}

	ok = true
	return
}
