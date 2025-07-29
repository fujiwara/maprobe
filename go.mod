module github.com/fujiwara/maprobe

go 1.24

toolchain go1.24.5

require (
	github.com/alecthomas/kong v1.4.0
	github.com/aws/aws-lambda-go v1.48.0
	github.com/aws/aws-sdk-go-v2 v1.25.1
	github.com/aws/aws-sdk-go-v2/config v1.27.0
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.15.0
	github.com/aws/aws-sdk-go-v2/service/firehose v1.28.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.48.0
	github.com/aws/aws-sdk-go-v2/service/ssm v1.49.0
	github.com/fujiwara/ridge v0.13.0
	github.com/fujiwara/sloghandler v0.0.5
	github.com/fujiwara/sloghandler/otelmetrics v0.0.5
	github.com/goccy/go-yaml v1.17.1
	github.com/google/go-cmp v0.7.0
	github.com/google/gops v0.3.28
	github.com/mackerelio/mackerel-client-go v0.37.0
	github.com/mattn/go-isatty v0.0.20
	github.com/shogo82148/go-retry v1.3.1
	github.com/tatsushid/go-fastping v0.0.0-20160109021039-d7bb493dee3e
	go.opentelemetry.io/otel v1.37.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.37.0
	go.opentelemetry.io/otel/metric v1.37.0
	go.opentelemetry.io/otel/sdk v1.37.0
	go.opentelemetry.io/otel/sdk/metric v1.37.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.15.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.16.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.19.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.22.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.27.0 // indirect
	github.com/aws/smithy-go v1.20.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/pires/go-proxyproto v0.8.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
