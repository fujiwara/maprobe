package maprobe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

var (
	DefaultGRPCTimeout         = 10 * time.Second
	DefaultGRPCMetricKeyPrefix = "grpc"
)

type GRPCProbeConfig struct {
	Address            string            `yaml:"address"`
	GRPCService        string            `yaml:"grpc_service"`
	Timeout            time.Duration     `yaml:"timeout"`
	TLS                bool              `yaml:"tls"`
	NoCheckCertificate bool              `yaml:"no_check_certificate"`
	Metadata           map[string]string `yaml:"metadata"`
	MetricKeyPrefix    string            `yaml:"metric_key_prefix"`
}

func (pc *GRPCProbeConfig) GenerateProbe(host *mackerel.Host) (Probe, error) {
	p := &GRPCProbe{
		hostID:             host.ID,
		metricKeyPrefix:    pc.MetricKeyPrefix,
		Timeout:            pc.Timeout,
		TLS:                pc.TLS,
		NoCheckCertificate: pc.NoCheckCertificate,
		Metadata:           make(map[string]string),
	}
	var err error

	p.Address, err = expandPlaceHolder(pc.Address, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	p.GRPCService, err = expandPlaceHolder(pc.GRPCService, host, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid grpc_service: %w", err)
	}

	for key, value := range pc.Metadata {
		p.Metadata[key], err = expandPlaceHolder(value, host, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata %s: %w", key, err)
		}
	}

	if p.Timeout == 0 {
		p.Timeout = DefaultGRPCTimeout
	}
	if p.metricKeyPrefix == "" {
		p.metricKeyPrefix = DefaultGRPCMetricKeyPrefix
	}

	return p, nil
}

type GRPCProbe struct {
	hostID          string
	metricKeyPrefix string

	Address            string
	GRPCService        string
	Timeout            time.Duration
	TLS                bool
	NoCheckCertificate bool
	Metadata           map[string]string
}

func (p *GRPCProbe) HostID() string {
	return p.hostID
}

func (p *GRPCProbe) MetricName(name string) string {
	return p.metricKeyPrefix + "." + name
}

func (p *GRPCProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *GRPCProbe) Run(ctx context.Context) (ms Metrics, err error) {
	var ok bool
	start := time.Now()
	defer func() {
		slog.Debug("grpc probe defer", "ok", ok)
		elapsed := time.Since(start)
		ms = append(ms, newMetric(p, "elapsed.seconds", elapsed.Seconds()))
		if ok {
			ms = append(ms, newMetric(p, "check.ok", 1))
		} else {
			ms = append(ms, newMetric(p, "check.ok", 0))
		}
		slog.Debug("grpc probe completed", "metrics", ms.String())
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	// Set up gRPC connection options
	var opts []grpc.DialOption
	if p.TLS {
		if p.NoCheckCertificate {
			config := &tls.Config{InsecureSkipVerify: true}
			creds := credentials.NewTLS(config)
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add metadata to context if provided
	if len(p.Metadata) > 0 {
		md := metadata.New(p.Metadata)
		timeoutCtx = metadata.NewOutgoingContext(timeoutCtx, md)
	}

	slog.Debug("dialing grpc", "address", p.Address)
	conn, err := grpc.DialContext(timeoutCtx, p.Address, opts...)
	if err != nil {
		return ms, fmt.Errorf("failed to dial: %w", err)
	}
	defer conn.Close()

	slog.Debug("connected", "address", p.Address)

	// Add certificate expiration metric for TLS connections
	if p.TLS {
		// Note: gRPC doesn't expose certificate details directly like net/http
		// This would require additional implementation if needed
		// For now, we'll skip certificate expiration for gRPC
	}

	// Create health check client
	healthClient := healthpb.NewHealthClient(conn)

	// Perform health check
	req := &healthpb.HealthCheckRequest{
		Service: p.GRPCService,
	}

	slog.Debug("health check", "grpc_service", p.GRPCService)
	resp, err := healthClient.Check(timeoutCtx, req)
	if err != nil {
		// Add gRPC status code if available
		if grpcErr, ok := err.(interface {
			GRPCStatus() interface{ Code() int }
		}); ok {
			statusCode := grpcErr.GRPCStatus().Code()
			ms = append(ms, newMetric(p, "status.code", float64(statusCode)))
		}
		return ms, fmt.Errorf("health check failed: %w", err)
	}

	// Add status code (0 = OK)
	ms = append(ms, newMetric(p, "status.code", 0))

	slog.Debug("health check response", "status", resp.Status)
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		return ms, fmt.Errorf("service not healthy: %v", resp.Status)
	}

	ok = true
	return
}
