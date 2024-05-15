package maprobe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	location string

	Probes            []*ProbeDefinition `yaml:"probes"`
	PostProbedMetrics bool               `yaml:"post_probed_metrics"`

	Aggregates            []*AggregateDefinition `yaml:"aggregates"`
	PostAggregatedMetrics bool                   `yaml:"post_aggregated_metrics"`

	ProbeOnly *bool `yaml:"probe_only"` // deprecated

	Backup      *BackupConfig      `yaml:"backup"`
	Destination *DestinationConfig `yaml:"destination"`
}

type MackerelConfig struct {
	Enabled bool `yaml:"enabled"`
}

type OtelConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
}

type DestinationConfig struct {
	Mackerel *MackerelConfig `yaml:"mackerel"`
	Otel     *OtelConfig     `yaml:"otel"`
}

type exString struct {
	Value string
}

func (s exString) String() string {
	return s.Value
}

func (s *exString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	err := unmarshal(&str)
	if err == nil {
		s.Value, err = expandPlaceHolder(str, nil, nil)
		return err
	}
	return err
}

func exStrings(es []exString) []string {
	ss := make([]string, 0, len(es))
	for _, s := range es {
		ss = append(ss, s.String())
	}
	return ss
}

type ProbeDefinition struct {
	Service  exString   `yaml:"service"`
	Role     exString   `yaml:"role"`
	Roles    []exString `yaml:"roles"`
	Statuses []exString `yaml:"statuses"`

	IsServiceMetric bool `yaml:"service_metric"`

	Ping    *PingProbeConfig    `yaml:"ping"`
	TCP     *TCPProbeConfig     `yaml:"tcp"`
	HTTP    *HTTPProbeConfig    `yaml:"http"`
	Command *CommandProbeConfig `yaml:"command"`

	Attributes map[string]string `yaml:"attributes"`
}

func (pd *ProbeDefinition) Validate() error {
	if pd.IsServiceMetric {
		if pd.Role.Value != "" || len(pd.Roles) > 0 || len(pd.Statuses) > 0 {
			return errors.Errorf("probe for service metric cannot have role or roles or statuses")
		}
	}
	return nil
}

func (pd *ProbeDefinition) GenerateProbes(host *mackerel.Host, client *mackerel.Client) []Probe {
	var probes []Probe

	if pingConfig := pd.Ping; pingConfig != nil {
		p, err := pingConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate ping probe. ID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	if tcpConfig := pd.TCP; tcpConfig != nil {
		p, err := tcpConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate tcp probe. ID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	if httpConfig := pd.HTTP; httpConfig != nil {
		p, err := httpConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate http probe. ID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	if commandConfig := pd.Command; commandConfig != nil {
		p, err := commandConfig.GenerateProbe(host, client)
		if err != nil {
			log.Printf("[error] cannot generate command probe. ID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	return probes
}

func LoadConfig(location string) (*Config, string, error) {
	c := &Config{
		location:              location,
		PostProbedMetrics:     true,
		PostAggregatedMetrics: true,
		Backup:                &BackupConfig{},
		Destination: &DestinationConfig{
			Mackerel: &MackerelConfig{
				Enabled: true,
			},
			Otel: &OtelConfig{
				Enabled: false,
			},
		},
	}
	b, err := c.fetch()
	if err != nil {
		return nil, "", errors.Wrap(err, "load config failed")
	}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, "", errors.Wrap(err, "yaml parse failed")
	}
	if err := c.initialize(); err != nil {
		return nil, "", errors.Wrap(err, "config initialize failed")
	}
	return c, fmt.Sprintf("%x", sha256.Sum256(b)), c.validate()
}

func (c *Config) initialize() error {
	// role -> roles
	for _, pd := range c.Probes {
		if pd.Role.String() != "" {
			pd.Roles = append(pd.Roles, pd.Role)
		}
		if pd.Command != nil {
			if err := pd.Command.initialize(); err != nil {
				return err
			}
		}
		if err := pd.Validate(); err != nil {
			return err
		}
	}
	for _, ad := range c.Aggregates {
		if r := ad.Role.String(); r != "" {
			ad.Roles = append(ad.Roles, ad.Role)
		}
	}
	return nil
}

func (c *Config) validate() error {
	if o := c.ProbeOnly; o != nil {
		log.Println("[warn] configuration probe_only is not deprecated. use post_probed_metrics")
		c.PostProbedMetrics = !*o
	}

	for _, ag := range c.Aggregates {
		for _, mc := range ag.Metrics {
			for _, oc := range mc.Outputs {
				switch strings.ToLower(oc.Func.String()) {
				case "sum":
					oc.calc = sum
				case "min", "minimum":
					oc.calc = min
				case "max", "maximum":
					oc.calc = max
				case "avg", "average":
					oc.calc = avg
				case "median":
					oc.calc = median
				case "count":
					oc.calc = count
				default:
					log.Printf(
						"[warn] func %s is not available for outputs %s",
						oc.Func, mc.Name,
					)
				}
			}
		}
	}

	return nil
}

func (c *Config) fetch() ([]byte, error) {
	u, err := url.Parse(c.location)
	if err != nil {
		// file path
		return os.ReadFile(c.location)
	}
	switch u.Scheme {
	case "http", "https":
		return fetchHTTP(u)
	case "s3":
		return fetchS3(u)
	default:
		// file
		return os.ReadFile(u.Path)
	}
}

func (c *Config) String() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func fetchHTTP(u *url.URL) ([]byte, error) {
	log.Println("[debug] fetching HTTP", u)
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func fetchS3(u *url.URL) ([]byte, error) {
	log.Println("[debug] fetching S3", u)
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	svc := s3.NewFromConfig(cfg)

	buf := &bytes.Buffer{}
	out, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get from S3, %w", err)
	}
	defer out.Body.Close()
	if _, err := io.Copy(buf, out.Body); err != nil {
		return nil, fmt.Errorf("failed to download from s3, %w", err)
	}
	return buf.Bytes(), nil
}

type AggregateDefinition struct {
	Service  exString        `yaml:"service"`
	Role     exString        `yaml:"role"`
	Roles    []exString      `yaml:"roles"`
	Statuses []exString      `yaml:"statuses"`
	Metrics  []*MetricConfig `yaml:"metrics"`
}

type MetricConfig struct {
	Name    exString        `yaml:"name"`
	Outputs []*OutputConfig `yaml:"outputs"`
}

type OutputConfig struct {
	Func     exString `yaml:"func"`
	Name     exString `yaml:"name"`
	EmitZero bool     `yaml:"emit_zero"`

	calc func([]float64) float64
}

type BackupConfig struct {
	FirehoseStreamName string `yaml:"firehose_stream_name"`
}
