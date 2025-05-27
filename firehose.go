package maprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/fujiwara/ridge"
)

const (
	commonAttrHeaderName = "X-Amz-Firehose-Common-Attributes"
	requestIDHeaderName  = "X-Amz-Firehose-Request-Id"
	accessKeyHeaderName  = "X-Amz-Firehose-Access-Key"
)

// firehoseCommonAttributes represents common attributes (metadata).
// https://docs.aws.amazon.com/ja_jp/firehose/latest/dev/httpdeliveryrequestresponse.html#requestformat
type firehoseCommonAttributes struct {
	CommonAttributes map[string]string `json:"commonAttributes"`
}

// firehoseRequestBody represents request body.
type firehoseRequestBody struct {
	RequestID string           `json:"requestId,omitempty"`
	Timestamp int64            `json:"timestamp,omitempty"`
	Records   []firehoseRecord `json:"records,omitempty"`
}

// firehoseRecord represents records in request body.
type firehoseRecord struct {
	Data []byte `json:"data"`
}

// firehoseResponseBody represents response body.
// https://docs.aws.amazon.com/ja_jp/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
type firehoseResponseBody struct {
	RequestID    string `json:"requestId,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// RunFirehoseEndpoint runs Firehose HTTP endpoint server.
func RunFirehoseEndpoint(ctx context.Context, wg *sync.WaitGroup, port int) {
	defer wg.Done()
	var mux = http.NewServeMux()
	mux.HandleFunc("/post", handleFirehoseRequest)
	mux.HandleFunc("/ping", handlePingRequest)
	ridge.RunWithContext(ctx, fmt.Sprintf(":%d", port), "/", mux)
}

func parseFirehoseRequest(r *http.Request) (*firehoseRequestBody, error) {
	accessKey := r.Header.Get(accessKeyHeaderName)
	if accessKey != MackerelAPIKey {
		return nil, fmt.Errorf("invalid access key")
	}

	var body firehoseRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %s", err)
	}
	return &body, nil
}

func handlePingRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/plain")
	fmt.Fprintln(w, "OK")
}

func handleFirehoseRequest(w http.ResponseWriter, r *http.Request) {
	slog.Info("[FirehoseEndpoint] accept HTTP request for Firhose Endpoint", "remoteAddr", r.RemoteAddr)
	w.Header().Add("content-type", "application/json")
	respBody := firehoseResponseBody{
		RequestID: r.Header.Get(requestIDHeaderName),
	}
	defer func() {
		respBody.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
		if e := respBody.ErrorMessage; e != "" {
			slog.Error("[FirehoseEndpoint]", "error", e)
		}
		json.NewEncoder(w).Encode(respBody)
	}()
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		respBody.ErrorMessage = "POST method required"
		return
	}

	reqBody, err := parseFirehoseRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		respBody.ErrorMessage = err.Error()
		return
	}

	client := newClient(MackerelAPIKey, "") // with no backup
	for _, record := range reqBody.Records {
		var payload backupPayload
		slog.Debug("[FirehoseEndpoint] record", "data", string(record.Data))
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			slog.Warn("[FirehoseEndpoint] failed to parse payload", "error", err)
			continue
		}
		if service := payload.Service; service != "" {
			slog.Info("[FirehoseEndpoint] post service metrics", "count", len(payload.MetricValues), "service", service)
			if err := client.PostServiceMetricValues(r.Context(), service, payload.MetricValues); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				respBody.ErrorMessage = err.Error()
				return
			}
		} else {
			slog.Info("[FirehoseEndpoint] post host metrics", "count", len(payload.HostMetricValues))
			if err := client.PostHostMetricValues(r.Context(), payload.HostMetricValues); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				respBody.ErrorMessage = err.Error()
				return
			}
		}
	}
}
