package maprobe

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"regexp"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
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
}

func (pc *TCPProbeConfig) GenerateProbe(host *mackerel.Host) (*TCPProbe, error) {
	p := &TCPProbe{
		hostID:             host.ID,
		Timeout:            pc.Timeout,
		MaxBytes:           pc.MaxBytes,
		TLS:                pc.TLS,
		NoCheckCertificate: pc.NoCheckCertificate,
	}
	var err error

	p.Host, err = expandPlaceHolder(pc.Host, host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid host")
	}

	p.Port, err = expandPlaceHolder(pc.Port, host)
	if err != nil {
		return nil, errors.Wrap(err, "invaild port")
	}

	p.Send, err = expandPlaceHolder(pc.Send, host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid send")
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
	hostID          string
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

func (p *TCPProbe) HostID() string {
	return p.hostID
}

func (p *TCPProbe) MetricName(name string) string {
	return p.metricKeyPrefix + "." + name
}

func (p *TCPProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *TCPProbe) Run(ctx context.Context) (ms Metrics, err error) {
	var ok bool
	start := time.Now()
	defer func() {
		log.Println("[debug] defer", ok)
		elapsed := time.Now().Sub(start)
		ms = append(ms, newMetric(p, "elapsed.seconds", elapsed.Seconds()))
		if ok {
			ms = append(ms, newMetric(p, "check.ok", 1))
		} else {
			ms = append(ms, newMetric(p, "check.ok", 0))
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	addr := net.JoinHostPort(p.Host, p.Port)

	log.Println("[debug] dialing", addr)
	conn, err := dialTCP(addr, p.TLS, p.NoCheckCertificate, p.Timeout)
	if err != nil {
		return ms, errors.Wrap(err, "connect failed")
	}
	defer conn.Close()

	log.Println("[debug] connected", addr)
	if p.Send != "" {
		log.Println("[debug] send", p.Send)
		_, err := io.WriteString(conn, p.Send)
		if err != nil {
			return ms, errors.Wrap(err, "send failed")
		}
	}
	if p.ExpectPattern != nil {
		buf := make([]byte, p.MaxBytes)
		r := bufio.NewReader(conn)
		n, err := r.Read(buf)
		if err != nil {
			return ms, errors.Wrap(err, "read failed")
		}
		log.Println("[debug] read", string(buf[:n]))

		if !p.ExpectPattern.Match(buf[:n]) {
			return ms, errors.Wrap(err, "unexpected response")
		}
	}
	if p.Quit != "" {
		log.Println("[debug]", p.Quit)
		io.WriteString(conn, p.Quit)
	}

	ok = true
	return
}

func dialTCP(address string, useTLS bool, noCheckCertificate bool, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	if useTLS {
		return tls.DialWithDialer(d, "tcp", address, &tls.Config{
			InsecureSkipVerify: noCheckCertificate,
		})
	}
	return d.Dial("tcp", address)
}
