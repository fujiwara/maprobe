package maprobe_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

var (
	GRPCServerAddress    string
	GRPCTLSServerAddress string
)

func init() {
	GRPCServerAddress = testGRPCServer()
	GRPCTLSServerAddress = testGRPCTLSServer()
}

func setupHealthServer() *health.Server {
	healthServer := health.NewServer()
	// Set specific service as SERVING
	healthServer.SetServingStatus("test.service", healthpb.HealthCheckResponse_SERVING)
	// Set another service as NOT_SERVING
	healthServer.SetServingStatus("unhealthy.service", healthpb.HealthCheckResponse_NOT_SERVING)
	// Server overall status is SERVING
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	return healthServer
}

func startGRPCServer(useTLS bool) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	var s *grpc.Server
	if useTLS {
		// Generate a test certificate that expires in 30 days
		cert, key := generateTestCertificate(30 * 24 * time.Hour)
		tlsCert := tls.Certificate{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  key,
		}
		config := &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}
		creds := credentials.NewTLS(config)
		s = grpc.NewServer(grpc.Creds(creds))
	} else {
		s = grpc.NewServer()
	}

	healthServer := setupHealthServer()
	healthpb.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(l); err != nil {
			serverType := "plain"
			if useTLS {
				serverType = "TLS"
			}
			log.Printf("Failed to serve %s gRPC: %v", serverType, err)
		}
	}()

	return l.Addr().String()
}

func testGRPCServer() string {
	return startGRPCServer(false)
}

func testGRPCTLSServer() string {
	return startGRPCServer(true)
}

func TestGRPCProbe(t *testing.T) {
	tests := []struct {
		name           string
		config         *maprobe.GRPCProbeConfig
		expectError    bool
		expectOK       bool
		expectedCode   *codes.Code
		expectedStatus float64
		minMetrics     int
	}{
		{
			name: "overall server health",
			config: &maprobe.GRPCProbeConfig{
				Address:     GRPCServerAddress,
				GRPCService: "",
				Timeout:     5 * time.Second,
			},
			expectError:    false,
			expectOK:       true,
			expectedStatus: 0,
			minMetrics:     3,
		},
		{
			name: "specific healthy service",
			config: &maprobe.GRPCProbeConfig{
				Address:     GRPCServerAddress,
				GRPCService: "test.service",
				Timeout:     5 * time.Second,
			},
			expectError:    false,
			expectOK:       true,
			expectedStatus: 0,
			minMetrics:     3,
		},
		{
			name: "unhealthy service",
			config: &maprobe.GRPCProbeConfig{
				Address:     GRPCServerAddress,
				GRPCService: "unhealthy.service",
				Timeout:     5 * time.Second,
			},
			expectError:    true,
			expectOK:       false,
			expectedStatus: 0,
			minMetrics:     3,
		},
		{
			name: "unknown service",
			config: &maprobe.GRPCProbeConfig{
				Address:     GRPCServerAddress,
				GRPCService: "unknown.service",
				Timeout:     5 * time.Second,
			},
			expectError:    true,
			expectOK:       false,
			expectedCode:   func() *codes.Code { c := codes.NotFound; return &c }(),
			expectedStatus: float64(codes.NotFound),
			minMetrics:     2,
		},
		{
			name: "connection failure",
			config: &maprobe.GRPCProbeConfig{
				Address:     "127.0.0.1:1", // Port 1 should fail
				GRPCService: "",
				Timeout:     2 * time.Second,
			},
			expectError: true,
			expectOK:    false,
			minMetrics:  2,
		},
		{
			name: "TLS connection",
			config: &maprobe.GRPCProbeConfig{
				Address:            GRPCTLSServerAddress,
				GRPCService:        "",
				Timeout:            5 * time.Second,
				TLS:                true,
				NoCheckCertificate: true, // Accept self-signed certificate
			},
			expectError:    false,
			expectOK:       true,
			expectedStatus: 0,
			minMetrics:     3,
		},
		{
			name: "TLS with specific service",
			config: &maprobe.GRPCProbeConfig{
				Address:            GRPCTLSServerAddress,
				GRPCService:        "test.service",
				Timeout:            5 * time.Second,
				TLS:                true,
				NoCheckCertificate: true,
			},
			expectError:    false,
			expectOK:       true,
			expectedStatus: 0,
			minMetrics:     3,
		},
		{
			name: "with metadata",
			config: &maprobe.GRPCProbeConfig{
				Address:     GRPCServerAddress,
				GRPCService: "",
				Timeout:     5 * time.Second,
				Metadata: map[string]string{
					"authorization": "Bearer test-token",
					"x-custom":      "value",
				},
			},
			expectError:    false,
			expectOK:       true,
			expectedStatus: 0,
			minMetrics:     3,
		},
		{
			name: "custom metric prefix",
			config: &maprobe.GRPCProbeConfig{
				Address:         GRPCServerAddress,
				GRPCService:     "",
				Timeout:         5 * time.Second,
				MetricKeyPrefix: "custom_grpc",
			},
			expectError:    false,
			expectOK:       true,
			expectedStatus: 0,
			minMetrics:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe, err := tt.config.GenerateProbe(&mackerel.Host{ID: "test", Name: "testhost"})
			if err != nil {
				t.Fatal(err)
			}

			ms, err := probe.Run(context.Background())

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("expected error, but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check specific error code if provided
			if tt.expectedCode != nil {
				if status.Code(err) != *tt.expectedCode {
					t.Errorf("expected error code %v, got: %v", *tt.expectedCode, status.Code(err))
				}
			}

			// Check minimum metrics count
			if len(ms) < tt.minMetrics {
				t.Errorf("unexpected metrics num: got %d, want at least %d", len(ms), tt.minMetrics)
			}

			// Check metric values
			prefix := "grpc"
			if tt.config.MetricKeyPrefix != "" {
				prefix = tt.config.MetricKeyPrefix
			}

			var foundCertMetric bool
			for _, m := range ms {
				switch m.Name {
				case prefix + ".elapsed.seconds":
					if m.Value < 0 {
						t.Error("elapsed time is negative")
					}
				case prefix + ".check.ok":
					expectedOK := float64(0)
					if tt.expectOK {
						expectedOK = 1
					}
					if m.Value != expectedOK {
						t.Errorf("check.ok = %f, want %f", m.Value, expectedOK)
					}
				case prefix + ".status.code":
					if tt.expectedStatus != 0 && m.Value != tt.expectedStatus {
						t.Errorf("status.code = %f, want %f", m.Value, tt.expectedStatus)
					}
				case prefix + ".certificate.expires_in_days":
					foundCertMetric = true
					// Should be around 30 days (certificate expires in 30 days)
					if m.Value < 29 || m.Value > 31 {
						t.Errorf("unexpected certificate expiration days: %f", m.Value)
					}
				}
			}
			if tt.config.TLS && !foundCertMetric {
				t.Error("certificate.expires_in_days metric not found")
			}
			t.Log(ms.String())
		})
	}
}

func TestGRPCProbeExpansion(t *testing.T) {
	pc := &maprobe.GRPCProbeConfig{
		Address:     "{{.Host.Name}}:{{.Host.ID}}",
		GRPCService: "{{.Host.CustomIdentifier}}",
		Metadata: map[string]string{
			"host-id": "{{.Host.ID}}",
		},
	}

	host := &mackerel.Host{
		ID:               "12345",
		Name:             "127.0.0.1",
		CustomIdentifier: "test.service",
	}

	probe, err := pc.GenerateProbe(host)
	if err != nil {
		t.Fatal(err)
	}

	// Check that placeholders were expanded
	grpcProbe := probe.(*maprobe.GRPCProbe)
	expectedAddress := fmt.Sprintf("%s:%s", host.Name, host.ID)
	if grpcProbe.Address != expectedAddress {
		t.Errorf("address not expanded correctly: got %s, want %s", grpcProbe.Address, expectedAddress)
	}
	if grpcProbe.GRPCService != host.CustomIdentifier {
		t.Errorf("grpc_service not expanded correctly: got %s, want %s", grpcProbe.GRPCService, host.CustomIdentifier)
	}
	if grpcProbe.Metadata["host-id"] != host.ID {
		t.Errorf("metadata not expanded correctly: got %s, want %s", grpcProbe.Metadata["host-id"], host.ID)
	}
}
