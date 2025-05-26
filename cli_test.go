package maprobe

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

func TestCLIParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envs     map[string]string
		expected interface{}
		wantErr  bool
	}{
		{
			name: "version command",
			args: []string{"version"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Version:     VersionCmd{},
			},
		},
		{
			name: "agent command with config",
			args: []string{"agent", "--config", "/path/to/config.yaml"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Agent: AgentCmd{
					Config:               "/path/to/config.yaml",
					WithFirehoseEndpoint: false,
					Port:                 8080,
				},
			},
		},
		{
			name: "agent command with environment variables",
			args: []string{"agent"},
			envs: map[string]string{
				"CONFIG":    "/env/config.yaml",
				"LOG_LEVEL": "debug",
				"GOPS":      "true",
			},
			expected: &CLI{
				LogLevel:    "debug",
				GopsEnabled: true,
				Agent: AgentCmd{
					Config:               "/env/config.yaml",
					WithFirehoseEndpoint: false,
					Port:                 8080,
				},
			},
		},
		{
			name: "agent command with all flags",
			args: []string{"agent", "-c", "/path/to/config.yaml", "--with-firehose-endpoint", "--port", "9090"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Agent: AgentCmd{
					Config:               "/path/to/config.yaml",
					WithFirehoseEndpoint: true,
					Port:                 9090,
				},
			},
		},
		{
			name: "once command",
			args: []string{"once", "--config", "/path/to/config.yaml"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Once: OnceCmd{
					Config: "/path/to/config.yaml",
				},
			},
		},
		{
			name: "lambda command",
			args: []string{"lambda", "-c", "/path/to/config.yaml"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Lambda: LambdaCmd{
					Config: "/path/to/config.yaml",
				},
			},
		},
		{
			name: "ping command basic",
			args: []string{"ping", "example.com"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Ping: PingCmd{
					Address: "example.com",
					Count:   0,
					Timeout: 0,
					HostID:  "",
				},
			},
		},
		{
			name: "ping command with all flags",
			args: []string{"ping", "example.com", "-c", "5", "-t", "30s", "-i", "host123"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				Ping: PingCmd{
					Address: "example.com",
					Count:   5,
					Timeout: 30 * time.Second,
					HostID:  "host123",
				},
			},
		},
		{
			name: "tcp command basic",
			args: []string{"tcp", "example.com", "80"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				TCP: TCPCmd{
					Host:               "example.com",
					Port:               "80",
					Send:               "",
					Quit:               "",
					Timeout:            0,
					ExpectPattern:      "",
					NoCheckCertificate: false,
					HostID:             "",
					TLS:                false,
				},
			},
		},
		{
			name: "tcp command with all flags",
			args: []string{"tcp", "example.com", "443", "-s", "GET / HTTP/1.1", "-q", "quit", "-t", "10s", "-e", "200 OK", "-k", "-i", "host123", "--tls"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				TCP: TCPCmd{
					Host:               "example.com",
					Port:               "443",
					Send:               "GET / HTTP/1.1",
					Quit:               "quit",
					Timeout:            10 * time.Second,
					ExpectPattern:      "200 OK",
					NoCheckCertificate: true,
					HostID:             "host123",
					TLS:                true,
				},
			},
		},
		{
			name: "http command basic",
			args: []string{"http", "https://example.com"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				HTTP: HTTPCmd{
					URL:                "https://example.com",
					Method:             "GET",
					Body:               "",
					ExpectPattern:      "",
					Timeout:            0,
					NoCheckCertificate: false,
					Headers:            nil,
					HostID:             "",
				},
			},
		},
		{
			name: "http command with all flags",
			args: []string{"http", "https://example.com", "-m", "POST", "-b", "data", "-e", "success", "-t", "30s", "-k", "-i", "host123"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				HTTP: HTTPCmd{
					URL:                "https://example.com",
					Method:             "POST",
					Body:               "data",
					ExpectPattern:      "success",
					Timeout:            30 * time.Second,
					NoCheckCertificate: true,
					Headers:            nil,
					HostID:             "host123",
				},
			},
		},
		{
			name: "firehose-endpoint command",
			args: []string{"firehose-endpoint", "-p", "9000"},
			expected: &CLI{
				LogLevel:    "info",
				GopsEnabled: false,
				FirehoseEndpoint: FirehoseEndpointCmd{
					Port: 9000,
				},
			},
		},
		{
			name: "global flags",
			args: []string{"--log-level", "warn", "--gops", "version"},
			expected: &CLI{
				LogLevel:    "warn",
				GopsEnabled: true,
				Version:     VersionCmd{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envs {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			var cli CLI
			parser, err := kong.New(&cli)
			if err != nil {
				t.Fatalf("Failed to create kong parser: %v", err)
			}

			ctx, err := parser.Parse(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error case
			}

			// Verify the command that was selected
			cmdName := ctx.Command()
			expected := tt.expected.(*CLI)

			// Check global flags
			if cli.LogLevel != expected.LogLevel {
				t.Errorf("LogLevel = %v, want %v", cli.LogLevel, expected.LogLevel)
			}
			if cli.GopsEnabled != expected.GopsEnabled {
				t.Errorf("GopsEnabled = %v, want %v", cli.GopsEnabled, expected.GopsEnabled)
			}

			// Check command-specific values
			switch cmdName {
			case "version":
				// Version command has no fields to check
			case "agent":
				if !reflect.DeepEqual(cli.Agent, expected.Agent) {
					t.Errorf("Agent = %+v, want %+v", cli.Agent, expected.Agent)
				}
			case "once":
				if !reflect.DeepEqual(cli.Once, expected.Once) {
					t.Errorf("Once = %+v, want %+v", cli.Once, expected.Once)
				}
			case "lambda":
				if !reflect.DeepEqual(cli.Lambda, expected.Lambda) {
					t.Errorf("Lambda = %+v, want %+v", cli.Lambda, expected.Lambda)
				}
			case "ping":
				if !reflect.DeepEqual(cli.Ping, expected.Ping) {
					t.Errorf("Ping = %+v, want %+v", cli.Ping, expected.Ping)
				}
			case "tcp":
				if !reflect.DeepEqual(cli.TCP, expected.TCP) {
					t.Errorf("TCP = %+v, want %+v", cli.TCP, expected.TCP)
				}
			case "http":
				if !reflect.DeepEqual(cli.HTTP, expected.HTTP) {
					t.Errorf("HTTP = %+v, want %+v", cli.HTTP, expected.HTTP)
				}
			case "firehose-endpoint":
				if !reflect.DeepEqual(cli.FirehoseEndpoint, expected.FirehoseEndpoint) {
					t.Errorf("FirehoseEndpoint = %+v, want %+v", cli.FirehoseEndpoint, expected.FirehoseEndpoint)
				}
			}
		})
	}
}

func TestCLIErrorCases(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "ping without address",
			args: []string{"ping"},
		},
		{
			name: "tcp without host",
			args: []string{"tcp"},
		},
		{
			name: "tcp without port",
			args: []string{"tcp", "example.com"},
		},
		{
			name: "http without url",
			args: []string{"http"},
		},
		{
			name: "invalid command",
			args: []string{"invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cli CLI
			parser, err := kong.New(&cli)
			if err != nil {
				t.Fatalf("Failed to create kong parser: %v", err)
			}

			_, err = parser.Parse(tt.args)
			if err == nil {
				t.Errorf("Expected error for args %v, but got none", tt.args)
			}
		})
	}
}