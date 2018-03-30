package maprove_test

import (
	"context"
	"testing"
	"time"

	"github.com/fujiwara/maprove"
)

var pingTimeout = 100 * time.Millisecond
var pingProves = []maprove.PingProve{
	maprove.PingProve{Address: "8.8.8.8", Count: 4, Timeout: pingTimeout},
	maprove.PingProve{Address: "google-public-dns-b.google.com", Count: 4, Timeout: pingTimeout},
}

func TestPing(t *testing.T) {
	for _, prove := range pingProves {
		ms, err := prove.Run(context.Background())
		if err != nil {
			t.Error(err)
		}
		t.Log(prove.Address, ms.String())
	}
}
