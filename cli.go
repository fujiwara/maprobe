package maprobe

import "time"

// CLI defines the command line interface structure for Kong
type CLI struct {
	LogLevel    string `name:"log-level" help:"log level" default:"info" env:"LOG_LEVEL"`
	LogFormat   string `name:"log-format" help:"log format (text|json)" default:"text" enum:"text,json" env:"LOG_FORMAT"`
	GopsEnabled bool   `name:"gops" help:"enable gops agent" default:"false" env:"GOPS"`

	Version          VersionCmd          `cmd:"" help:"Show version"`
	Agent            AgentCmd            `cmd:"" help:"Run agent"`
	Once             OnceCmd             `cmd:"" help:"Run once"`
	Lambda           LambdaCmd           `cmd:"" help:"Run on AWS Lambda like once mode"`
	Ping             PingCmd             `cmd:"" help:"Run ping probe"`
	TCP              TCPCmd              `cmd:"" help:"Run TCP probe"`
	HTTP             HTTPCmd             `cmd:"" help:"Run HTTP probe"`
	GRPC             GRPCCmd             `cmd:"" help:"Run gRPC probe"`
	FirehoseEndpoint FirehoseEndpointCmd `cmd:"" help:"Run Firehose HTTP endpoint"`
}

// VersionCmd represents the version command
type VersionCmd struct{}

// AgentCmd represents the agent command that runs continuously
type AgentCmd struct {
	Config               string `short:"c" help:"configuration file path or URL(http|s3)" env:"CONFIG"`
	WithFirehoseEndpoint bool   `help:"run with firehose HTTP endpoint server"`
	Port                 int    `help:"firehose HTTP endpoint listen port" default:"8080"`
}

// OnceCmd represents the once command that runs probes once and exits
type OnceCmd struct {
	Config string `short:"c" help:"configuration file path or URL(http|s3)" env:"CONFIG"`
}

// LambdaCmd represents the lambda command that runs on AWS Lambda
type LambdaCmd struct {
	Config string `short:"c" help:"configuration file path or URL(http|s3)" env:"CONFIG"`
}

// PingCmd represents the ping command for standalone ping probe
type PingCmd struct {
	Address string        `arg:"" help:"Hostname or IP address" required:""`
	Count   int           `short:"c" help:"Iteration count"`
	Timeout time.Duration `short:"t" help:"Timeout to ping response"`
	HostID  string        `short:"i" help:"Mackerel host ID"`
}

// TCPCmd represents the TCP command for standalone TCP probe
type TCPCmd struct {
	Host               string        `arg:"" help:"Hostname or IP address" required:""`
	Port               string        `arg:"" help:"Port number" required:""`
	Send               string        `short:"s" help:"String to send to the server"`
	Quit               string        `short:"q" help:"String to send server to initiate a clean close of the connection"`
	Timeout            time.Duration `short:"t" help:"Timeout"`
	ExpectPattern      string        `short:"e" name:"expect" help:"Regexp pattern to expect in server response"`
	NoCheckCertificate bool          `short:"k" help:"Do not check certificate"`
	HostID             string        `short:"i" help:"Mackerel host ID"`
	TLS                bool          `help:"Use TLS"`
}

// HTTPCmd represents the HTTP command for standalone HTTP probe
type HTTPCmd struct {
	URL                string            `arg:"" help:"URL" required:""`
	Method             string            `short:"m" help:"Request method" default:"GET"`
	Body               string            `short:"b" help:"Request body"`
	ExpectPattern      string            `short:"e" name:"expect" help:"Regexp pattern to expect in server response"`
	Timeout            time.Duration     `short:"t" help:"Timeout"`
	NoCheckCertificate bool              `short:"k" help:"Do not check certificate"`
	Headers            map[string]string `short:"H" name:"header" help:"Request headers" placeholder:"Header: Value"`
	HostID             string            `short:"i" help:"Mackerel host ID"`
}

// GRPCCmd represents the gRPC command for standalone gRPC probe
type GRPCCmd struct {
	Address            string            `arg:"" help:"gRPC server address (host:port)" required:""`
	GRPCService        string            `short:"s" name:"service" help:"gRPC service name for health check"`
	Timeout            time.Duration     `short:"t" help:"Timeout"`
	NoCheckCertificate bool              `short:"k" help:"Do not check certificate"`
	Metadata           map[string]string `short:"m" name:"metadata" help:"gRPC metadata" placeholder:"key:value"`
	HostID             string            `short:"i" help:"Mackerel host ID"`
	TLS                bool              `help:"Use TLS"`
}

// FirehoseEndpointCmd represents the firehose endpoint command for HTTP server
type FirehoseEndpointCmd struct {
	Port int `short:"p" help:"Listen port" default:"8080"`
}
