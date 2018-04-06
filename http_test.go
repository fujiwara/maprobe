package maprobe_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var HTTPServerURL string

func testHTTPServer() string {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(w, "Hello HTTP Test")
	})
	ts := httptest.NewServer(handler)
	return ts.URL
}

func TestHTTP(t *testing.T) {
	pc := &maprobe.HTTPProbeConfig{
		URL:           HTTPServerURL,
		ExpectPattern: "^Hello",
	}

	probe, err := pc.GenerateProbe(&mackerel.Host{ID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	log.Println(probe, err)

	ms, err := probe.Run(context.Background())
	if err != nil {
		t.Error(err)
	}

	if len(ms) != 3 {
		t.Error("unexpected metrics num")
	}
	for _, m := range ms {
		switch m.Name {
		case "http.respose_time.seconds":
			if m.Value < 0.1 {
				t.Error("elapsed time too short")
			}
		case "http.check.ok":
			if m.Value != 1 {
				t.Error("check failed")
			}
		case "http.status.code":
			if m.Value != 200 {
				t.Errorf("unexpected status %f", m.Value)
			}
		}
	}
	t.Log(ms.String())
}
