package maprobe_test

import (
	"context"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
)

func TestDoRetry(t *testing.T) {
	t.Setenv("EMULATE_FAILURE", "true")
	client := maprobe.NewClient("dummy", "")
	tries := 0
	start := time.Now()
	err := maprobe.DoRetry(context.Background(), func() error {
		tries++
		return client.PostHostMetricValues(nil)
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Errorf("error expected")
	}
	if tries <= 1 {
		t.Errorf("retry expected")
	}
	if elapsed < 10*time.Second {
		t.Errorf("retry delay expected")
	}
}
