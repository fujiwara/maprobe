package maprobe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	location string
	source   []byte

	APIKey    string             `yaml:"apikey"`
	Probes    []*ProbeDefinition `yaml:"probes"`
	ProbeOnly bool               `yaml:"probe_only"`
}

type ProbeDefinition struct {
	Service string              `yaml:"service"`
	Role    string              `yaml:"role"`
	Roles   []string            `yaml:"roles"`
	Ping    *PingProbeConfig    `yaml:"ping"`
	TCP     *TCPProbeConfig     `yaml:"tcp"`
	HTTP    *HTTPProbeConfig    `yaml:"http"`
	Command *CommandProbeConfig `yaml:"command"`
}

func (pc *ProbeDefinition) GenerateProbes(host *mackerel.Host) []Probe {
	var probes []Probe

	if pingConfig := pc.Ping; pingConfig != nil {
		p, err := pingConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate ping probe. HostID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	if tcpConfig := pc.TCP; tcpConfig != nil {
		p, err := tcpConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate tcp probe. HostID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	if httpConfig := pc.HTTP; httpConfig != nil {
		p, err := httpConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate http probe. HostID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	if commandConfig := pc.Command; commandConfig != nil {
		p, err := commandConfig.GenerateProbe(host)
		if err != nil {
			log.Printf("[error] cannot generate command probe. HostID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			probes = append(probes, p)
		}
	}

	return probes
}

func LoadConfig(location string) (*Config, error) {
	c := &Config{
		location: location,
		APIKey:   os.Getenv("MACKEREL_APIKEY"),
	}
	b, err := c.fetch()
	if err != nil {
		return nil, errors.Wrap(err, "load config failed")
	}
	c.source = b
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}
	c.initialize()
	return c, c.validate()
}

func (c *Config) initialize() {
	for _, pd := range c.Probes {
		if pd.Role != "" {
			pd.Roles = append(pd.Roles, pd.Role)
		}
	}
}

func (c *Config) validate() error {
	if c.APIKey == "" {
		return errors.New("no API Key")
	}
	return nil
}

func (c *Config) fetch() ([]byte, error) {
	u, err := url.Parse(c.location)
	if err != nil {
		// file path
		return ioutil.ReadFile(c.location)
	}
	switch u.Scheme {
	case "http", "https":
		return fetchHTTP(u)
	case "s3":
		return fetchS3(u)
	default:
		// file
		return ioutil.ReadFile(u.Path)
	}
}

func (c *Config) String() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c *Config) Reload() (*Config, bool, error) {
	log.Println("[debug] reload config")
	b, err := c.fetch()
	if err != nil {
		return c, false, errors.Wrap(err, "failed to fetch config")
	}
	if bytes.Equal(b, c.source) {
		// not changed
		return c, false, nil
	}
	log.Println("[info] new config available. reloading")
	c2 := &Config{
		location: c.location,
		source:   b,
		APIKey:   os.Getenv("MACKEREL_APIKEY"),
	}
	if err := yaml.Unmarshal(b, c2); err != nil {
		return c, false, errors.Wrap(err, "failed to load a new config")
	}
	c2.initialize()
	return c2, true, nil
}

func fetchHTTP(u *url.URL) ([]byte, error) {
	log.Println("[debug] fetching HTTP", u)
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func fetchS3(u *url.URL) ([]byte, error) {
	log.Println("[debug] fetching S3", u)
	sess := session.Must(session.NewSession())
	downloader := s3manager.NewDownloader(sess)

	buf := &aws.WriteAtBuffer{}
	_, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from S3, %s", err)
	}
	return buf.Bytes(), nil
}
