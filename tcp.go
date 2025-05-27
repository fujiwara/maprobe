package maprobe

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

var (
	DefaultTCPTimeout         = 5 * time.Second
	DefaultTCPMaxBytes        = 32 * 1024
	DefaultTCPMetricKeyPrefix = "tcp"
)

type TCPProbeConfig struct {
	Host               string        `yaml:"host"`
	Port               string        `yaml:"port"`
	Timeout            time.Duration `yaml:"timeout"`
	Send               string        `yaml:"send"`
	Quit               string        `yaml:"quiet"`
	MaxBytes           int           `yaml:"max_bytes"`
	ExpectPattern      string        `yaml:"expect_pattern"`
	TLS                bool          `yaml:"tls"`
	NoCheckCertificate bool          `yaml:"no_check_certificate"`
	MetricKeyPrefix    string        `yaml:"metric_key_prefix"`
}

func (pc *TCPProbeConfig) GenerateProbe(host *mackerel.Host) (Probe, error) {
	p := &TCPProbe{
		metricKeyPrefix:    pc.MetricKeyPrefix,
		Timeout:            pc.Timeout,
		MaxBytes:           pc.MaxBytes,
		TLS:                pc.TLS,
		NoCheckCertificate: pc.NoCheckCertificate,
	}
	var err error

	p.Host, err = expandPlaceHolder(pc.Host, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid host: %w", err)
	}

	p.Port, err = expandPlaceHolder(pc.Port, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	p.Send, err = expandPlaceHolder(pc.Send, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid send: %w", err)
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

	if p.Timeout == 0 {
		p.Timeout = DefaultTCPTimeout
	}
	if p.MaxBytes == 0 {
		p.MaxBytes = DefaultTCPMaxBytes
	}
	if p.metricKeyPrefix == "" {
		p.metricKeyPrefix = DefaultTCPMetricKeyPrefix
	}

	return p, nil
}

type TCPProbe struct {
	metricKeyPrefix string

	Host               string
	Port               string
	Send               string
	Quit               string
	MaxBytes           int
	ExpectPattern      *regexp.Regexp
	Timeout            time.Duration
	TLS                bool
	NoCheckCertificate bool
}

func (p *TCPProbe) MetricName(name string) string {
	return p.metricKeyPrefix + "." + name
}

func (p *TCPProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *TCPProbe) Run(_ context.Context) (ms Metrics, err error) {
	var ok bool
	start := time.Now()
	defer func() {
		slog.Debug("tcp probe defer", "ok", ok)
		elapsed := time.Since(start)
		ms = append(ms, newMetric(p, "elapsed.seconds", elapsed.Seconds()))
		if ok {
			ms = append(ms, newMetric(p, "check.ok", 1))
		} else {
			ms = append(ms, newMetric(p, "check.ok", 0))
		}
		slog.Debug("tcp probe completed", "metrics", ms.String())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()

	addr := net.JoinHostPort(p.Host, p.Port)

	slog.Debug("dialing", "addr", addr)
	conn, err := dialTCP(ctx, addr, p.TLS, p.NoCheckCertificate, p.Timeout)
	if err != nil {
		return ms, fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(p.Timeout))

	slog.Debug("connected", "addr", addr)
	if p.Send != "" {
		slog.Debug("send", "data", p.Send)
		_, err := io.WriteString(conn, p.Send)
		if err != nil {
			return ms, fmt.Errorf("send failed: %w", err)
		}
	}
	if p.ExpectPattern != nil {
		buf := make([]byte, p.MaxBytes)
		r := bufio.NewReader(conn)
		n, err := r.Read(buf)
		if err != nil {
			return ms, fmt.Errorf("read failed: %w", err)
		}
		slog.Debug("read", "data", string(buf[:n]))

		if !p.ExpectPattern.Match(buf[:n]) {
			return ms, fmt.Errorf("unexpected response")
		}
	}
	if p.Quit != "" {
		slog.Debug("quit", "data", p.Quit)
		io.WriteString(conn, p.Quit)
	}

	ok = true
	return
}

func dialTCP(ctx context.Context, address string, useTLS bool, noCheckCertificate bool, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	if useTLS {
		td := &tls.Dialer{
			NetDialer: d,
			Config: &tls.Config{
				InsecureSkipVerify: noCheckCertificate,
			},
		}
		return td.DialContext(ctx, "tcp", address)
	}
	return d.DialContext(ctx, "tcp", address)
}
