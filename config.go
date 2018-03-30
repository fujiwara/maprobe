package maprove

import (
	"io/ioutil"
	"log"
	"os"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	APIKey       string         `yaml:"apikey"`
	ProvesConfig []*ProveConfig `yaml:"proves"`
}

type ProveConfig struct {
	Service string           `yaml:"service"`
	Role    string           `yaml:"role"`
	Roles   []string         `yaml:"roles"`
	Ping    *PingProveConfig `yaml:"ping"`
}

func (pc *ProveConfig) Proves(host *mackerel.Host) []Prove {
	var proves []Prove
	if ping := pc.Ping; ping != nil {
		p, err := ping.Prove(host)
		if err != nil {
			log.Printf("[error] cannot generate ping prove. HostID:%s Name:%s %s", host.ID, host.Name, err)
		} else {
			proves = append(proves, p)
		}
	}
	return proves
}

func LoadConfig(path string) (*Config, error) {
	c := Config{
		APIKey: os.Getenv("MACKEREL_APIKEY"),
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "load config failed")
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	for _, pc := range c.ProvesConfig {
		if pc.Role != "" {
			pc.Roles = append(pc.Roles, pc.Role)
		}
	}
	return &c, nil
}
