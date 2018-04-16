package maprobe_test

import (
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
