package maprove_test

import (
	"testing"

	"github.com/fujiwara/maprove"
)

func TestConfig(t *testing.T) {
	conf, err := maprove.LoadConfig("test/config.yaml")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%#v", conf)
}
