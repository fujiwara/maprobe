package maprobe_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/fujiwara/maprobe"
)

func TestConfig(t *testing.T) {
	conf, err := maprobe.LoadConfig("test/config.yaml")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%#v", conf)
}

func TestConfigReload(t *testing.T) {
	path := "test/test_generated.yaml"
	defer os.Remove(path)
	err := ioutil.WriteFile(path, []byte(`
apikey: dummy
probes:
  - service: production
    role: EC2
    ping:
      address: "{{ .Host.IPAddresses.eth0 }}"
`), 0644)
	if err != nil {
		t.Error(err)
	}
	conf, err := maprobe.LoadConfig(path)
	if err != nil {
		t.Error(err)
	}

	if len(conf.Probes) != 1 {
		t.Errorf("unexpected probes sections %d", len(conf.Probes))
	}
	if conf.Probes[0].Service != "production" || conf.Probes[0].Role != "EC2" || conf.Probes[0].Ping == nil {
		t.Errorf("unexpected probe section %v", conf.Probes[0])
	}

	err = ioutil.WriteFile("test/test_generated.yaml", []byte(`
apikey: dummy
probes:
  - service: staging
    role: EC2
    ping:
      address: "{{ .Host.IPAddresses.eth0 }}"
  - service: staging
    role: EC2
    tcp:
      host: "{{ .Host.IPAddresses.eth0 }}"
      port: 9999
`), 0644)
	if err != nil {
		t.Error(err)
	}

	var reloaded bool
	conf, reloaded, err = conf.Reload()
	if !reloaded {
		t.Error("config must be reloaded")
	}
	if err != nil {
		t.Error(err)
	}

	if len(conf.Probes) != 2 {
		t.Errorf("unexpected probes sections %d", len(conf.Probes))
	}
	if conf.Probes[0].Service != "staging" || conf.Probes[0].Role != "EC2" || conf.Probes[0].Ping == nil {
		t.Errorf("unexpected probe section %v", conf.Probes[0])
	}
	if conf.Probes[1].Service != "staging" || conf.Probes[1].Role != "EC2" || conf.Probes[1].TCP == nil {
		t.Errorf("unexpected probe section %v", conf.Probes[1])
	}
}
