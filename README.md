# maprobe

Mackerel external probe / aggregate agent.

## Description

maprobe is an external probe / aggregate agent with [Mackerel](https://mackerel.io).

maprobe agent works as below.

### for probes

1. Fetch hosts information from Mackerel API.
   - Filtered service and role.
1. For each hosts, execute probes (ping, tcp, http, command).
   - expand place holder in configuration `{{ .Host }}` as [Mackerel host struct](https://godoc.org/github.com/mackerelio/mackerel-client-go#Host).
   - `{{ .Host.IPAddress.eth0 }}` expand to e.g. `192.168.1.1`
1. Posts host metrics to Mackerel (and/or OpenTelemetry metrics endpoint if configured).
1. Iterates these processes each 60 sec.

### for aggregates

1. Fetch hosts information from Mackerel API.
   - Filtered service and role.
1. For each hosts, fetch specified host metrics to calculates these metrics by functions.
1. Post theses aggregated metrics as Mackerel service metrics.
1. Iterates these processes each 60 sec.

## Install

### Binary packages

[GitHub releases](https://github.com/fujiwara/maprobe/releases)

### Docker

[DockerHub](https://hub.docker.com/r/fujiwara/maprobe/)

## Usage

```
usage: maprobe [<flags>] <command> [<args> ...]

Flags:
  --help              Show context-sensitive help (also try --help-long and --help-man).
  --log-level="info"  log level

Commands:
  help [<command>...]
    Show help.

  agent [<flags>]
    Run agent

  once [<flags>]
    Run once

  ping [<flags>] <address>
    Run ping probe

  tcp [<flags>] <host> <port>
    Run TCP probe

  http [<flags>] <url>
    Run HTTP probe

  grpc [<flags>] <address>
    Run gRPC probe

  firehose-endpoint [<flags>]
    Run Firehose HTTP endpoint
```

### agent / once

`MACKEREL_APIKEY` environment variable is required.

```
$ maprobe agent --help
usage: maprobe agent [<flags>]

Run agent

Flags:
      --help              Show context-sensitive help (also try --help-long and --help-man).
      --log-level="info"  log level
  -c, --config=CONFIG     configuration file path or URL(http|s3)
```

`--config` accepts a local file path or URL(http, https or s3 scheme).
maprobe checks the config is modified, and reload in run time.

Defaults of `--config` and `--log-level` will be overrided from envrionment variables (`CONFIG` and `LOG_LEVEL`).

`agent` runs maprobe forever, `once` runs maprobe once.

### Example Configuration for probes

```yaml
post_probed_metrics: false   # when false, do not post host metrics to Mackerel. only dump to [info] log.
probes:
  - service: '{{ env "SERVICE" }}'   # expand environment variable
    role: server
    ping:
      address: '{{ .Host.IPAddresses.eth0 }}'

  - service: production
    role: webserver
    http:
      url: 'http://{{ .Host.CustomIdentifier }}/api/healthcheck'
      post: POST
      headers:
        Content-Type: application/json
      body: '{"hello":"world"}'
      expect_pattern: 'ok'

  - service: production
    role: redis
    tcp:
      host: '{{ .Host.IPAddress.eth0 }}'
      port: 6379
      send: "PING\n"
      expect_pattern: "PONG"
      quit: "QUIT\n"
    command:
      command:
        - "mackerel-plugin-redis"
        - "-host={{ .Host.IPAddress.eth0 }}"
        - "-tempfile=/tmp/redis-{{ .Host.ID }}"
    attributes: # supoort OpenTelemetry attributes
      - service.namespaece: redis
      - host.name: "{{ .Host.Name }}"

  - service: production
    role: api
    grpc:
      address: '{{ .Host.IPAddress.eth0 }}:50051'
      grpc_service: "api.v1.UserService"
      metadata:
        authorization: "Bearer {{ env "API_TOKEN" }}"
      tls: true

  - service: production
    service_metric: true # post metrics as service metrics
    http:
      url: 'https://example.net/api/healthcheck'
      post: GET
      headers:
        Content-Type: application/json
      body: '{"hello":"world"}'
      expect_pattern: 'ok'

destination:
  mackerel:
    enabled: true # default true
  otel:
    enabled: true # default false
    endpoint: localhost:4317
    insecure: true
```

#### OpenTelemetry metrics endpoint support

`destination.otel.enabled: true` enables to post metrics to OpenTelemetry metrics endpoint.
maprobe uses the gRPC protocol to send metrics.

```yaml
destination:
  mackerel:
    enabled: false # disable mackerel host/service metrics
  otel:
    enabled: true
    endpoint: localhost:4317
    insecure: true
    resource_attributes: # OpenTelemetry resource attributes (for all metrics)
      deployment.environment.name: production
    stats_attributes: # for maprobe internal metrics
      service.name: maprobe
      service.namespace: my-namespace
```

Extra attributes can be added to metrics by `attributes` in probe configuration.

By default, maprobe adds `service.name` and `host.id` attributes to metrics.

```yaml
probes:
  - service: production
    role: redis
    command:
      command:
        - "mackerel-plugin-redis"
        - "-host={{ .Host.IPAddress.eth0 }}"
        - "-tempfile=/tmp/redis-{{ .Host.ID }}"
    attributes: # extra attributes
      - service.namespace: redis
      - host.name: "{{ .Host.Name }}"
```

#### Service metrics support in probes

`service_metric: true` in probe configuration enables to post metrics as service metrics.

```yaml
probes:
  - service: production
    service_metric: true # post metrics as service metrics
    # ...
```

In this case, `.Host` is not available in probe configuration.

### Backup metrics using Amazon Kinesis Firehose

When Mackerel API is down, maprobe can backup corrected metrics to Amazon Kinesis Firehose.

```yaml
backup:
  firehose_stream_name: your-maprobe-backup
```

If maprobe cannot post metrics to Mackerel API, maprobe posts these metrics to Firehose stream as backup.

`maprobe agent --with-firehose-endpoint` or `maprobe firehose-endpoint` runs HTTP server for [Firehose HTTP Endpoint](https://docs.aws.amazon.com/firehose/latest/dev/create-destination.html#create-destination-http).

You can configure the Firehose stream that send data to HTTP endpoint to maprobe's http server.

```
[maprobe] -XXX-> [Mackerel]
          \
        (backup)
            \---> [Firehose](buffer and retry) -(ELB)-> [maprobe HTTP] --> [Mackerel]
```

Firehose HTTP Endpoint has paths below.
- `/post` : Post metrics endpoint.
  "Access key" must be same the as MACKEREL_APIKEY which set in maprobe.
- `/ping` : Always return 200 OK (for health check).

maprobe accepts Firehose HTTP requests and the metrics will send to Mackerel API (when available).

### Ping

Ping probe sends ICMP ping to the address.

```yaml
ping:
  address: "192.168.1.1"      # Hostname or IP address (required)
  count: 5                    # Iteration count (default 3)
  timeout: "500ms"            # Timeout to ping response (default 1 sec)
  metric_key_prefix:          # default ping
```

Ping probe generates the following metrics.

- ping.count.success (count)
- ping.count.failure (count)
- ping.rtt.min (seconds)
- ping.rtt.max (seconds)
- ping.rtt.avg (seconds)

### TCP

TCP probe connects to host:port by TCP (or TLS).

```yaml
tcp:
  host: "memcached.example.com" # Hostname or IP Address (required)
  port: 11211                   # Port number (required)
  timeout: 10s                  # Seconds of timeout (default 5)
  send: "VERSION\n"             # String to send to the server
  quit: "QUIT\n"                # String to send server to initiate a clean close of the connection"
  expect_pattern: "^VERSION 1"  # Regexp pattern to expect in server response
  tls: false                    # Use TLS for connection
  no_check_certificate: false   # Do not check certificate
  metric_key_prefix:            # default tcp
```

TCP probe generates the following metrics.

- tcp.check.ok (0 or 1)
- tcp.elapsed.seconds (seconds)
- tcp.certificate.expires_in_days (days until TLS certificate expires, only for TLS connections)

### HTTP

HTTP probe sends a HTTP request to url.

```yaml
http:
  url: "http://example.com/"     # URL
  method: "GET"                  # Method of request (default GET)
  headers:                       # Map of request header
    Foo: "bar"
  body: ""                       # Body of request
  expect_pattern: "ok"           # Regexp pattern to expect in server response
  timeout: 10s                   # Seconds of request timeout (default 15)
  no_check_certificate: false    # Do not check certificate
  metric_key_prefix:             # default http
```

HTTP probe generates the following metrics.

- http.check.ok (0 or 1)
- http.response_time.seconds (seconds)
- http.status.code (100~)
- http.content.length (bytes)
- http.certificate.expires_in_days (days until SSL/TLS certificate expires, only for HTTPS URLs)

When a status code is grather than 400, http.check.ok set to 0.

### gRPC

gRPC probe checks the health of a gRPC service using the standard gRPC Health Checking Protocol.

```yaml
grpc:
  address: "localhost:50051"     # gRPC server address (required)
  grpc_service: ""               # Service name for health check (empty for overall server health)
  timeout: 10s                   # Timeout (default 10s)
  tls: false                     # Use TLS for connection
  no_check_certificate: false    # Do not check certificate
  metadata:                      # gRPC metadata (for authentication, etc.)
    authorization: "Bearer token"
  metric_key_prefix:             # default grpc
```

gRPC probe generates the following metrics.

- grpc.check.ok (0 or 1)
- grpc.elapsed.seconds (seconds)
- grpc.status.code (gRPC status code, 0 = OK)

The probe uses the standard [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md). When `grpc_service` is empty, it checks the overall server health. When specified, it checks the health of that specific service.

### Command

Command probe executes command which outputs like Mackerel metric plugin.

```yaml
command:
  command: "/path/to/metric-command -option=foo" # execute command
  timeout: "5s"                      # Seconds of command timeout (default 15)
  graph_defs: true                   # Post graph definitions to Mackerel (default false)
  env:  # environment variables for command execution
    FOO: foo
    BAR: bar
```

`command` accepts both a single string value and an array value. If an array value is passed, these are not processed by shell.

```yaml
command:
  command:
    - "/path/to/metric-command"
    - "-option=foo"
  timeout: "5s"                      # Seconds of command timeout (default 15)
  graph_defs: true                   # Post graph definitions to Mackerel (default false)
```

Command probe handles command's output as host metric.

When `graph_defs` is true, maprobe runs a command with `MACKEREL_AGENT_PLUGIN_META=1` environment variables and post graph definitions to Mackerel at first time.

If the command does not return a valid graph definitions output, that is ignored.

See also [ホストのカスタムメトリックを投稿する - Mackerel ヘルプ](https://mackerel.io/ja/docs/entry/advanced/custom-metrics#graph-schema).

#### Example of automated cleanup for terminated EC2 instances.

Command probe can run any scripts against for Mackerel hosts.

For example,

```yaml
service: production
role: server
statues:
  - working
  - standby
  - poweroff
command:
  command: 'cleanup.sh {{.Host.ID}} {{index .Host.Meta.Cloud.MetaData "instance-id"}}'
```

cleanup.sh checks an instance status, retire a Mackerel host when the instance is not exists.

```bash
#!/bin/bash
set -u
host_id="$1"
instance_id="$2"
exec 1> /dev/null # dispose stdout
result=$(aws ec2 describe-instance-status --instance-id "${instance_id}" 2>&1)
if [[ $? == 0 ]]; then
  exit
elif [[ $result =~ "InvalidInstanceID.NotFound" ]]; then
   mkr retire --force "${host_id}"
fi
```

### Example Configuration for aggregates

```yaml
post_aggregated_metrics: false   # when false, do not post service metrics to Mackerel. only dump to [info] log.
aggregates:
  - service: production
    role: app-server
    metrics:
      - name: cpu.user.percentage
        outputs:
          - func: sum
            name: cpu.user.sum_percentage
          - func: avg
            name: cpu.user.avg_percentage
      - name: cpu.idle.percentage
        outputs:
          - func: sum
            name: cpu.idle.sum_percentage
          - func: avg
            name: cpu.idle.avg_percentage
```

This configuration posts service metrics (for service "production") as below.

- cpu.user.sum_percentage = sum(cpu.user.percentage) of production:app-server
- cpu.user.avg_percentage = avg(cpu.user.percentage) of production:app-server
- cpu.idle.sum_percentage = sum(cpu.idle.percentage) of production:app-server
- cpu.idle.avg_percentage = avg(cpu.idle.percentage) of production:app-server

#### functions for aggregates

Following functions are available to aggregate host metrics.

- sum
- min / minimum
- max / maximum
- avg / average
- median
- count

## Author

Fujiwara Shunichiro <fujiwara.shunichiro@gmail.com>

## License

Copyright 2018 Fujiwara Shunichiro

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

nless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
