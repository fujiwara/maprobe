# maprobe

Mackerel external probe agent.

## Description

maprobe is an external probe agent with [Mackerel](https://mackerel.io).

maprobe works as below.

1. Fetch hosts information from Mackerel API.
   - Filtered service and role.
1. For each hosts, execute probes (ping, tcp, http, command).
   - expand place holder in configuration `{{ .Host }}` as [Mackerel host struct](https://godoc.org/github.com/mackerelio/mackerel-client-go#Host).
   - `{{ .Host.IPAddress.eth0 }}` expand to e.g. `192.168.1.1`
1. Post host metrics to Mackerel.
1. Iterates these processes each 60 sec.

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

  ping [<flags>] <address>
    Run ping probe

  tcp [<flags>] <host> <port>
    Run TCP probe

  http [<flags>] <url>
    Run HTTP probe
```

### agent

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

### Example Configuration

```yaml
probe_only: false   # when true, do not post metrics to Mackerel. only dump to debug log.
probes:
  - service: production
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
      command: "mackerel-plugin-redis -host {{ .Host.IPAddress.eth0 }} -tempfile /tmp/redis-{{ .Host.ID }}"
```

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

When a status code is grather than 400, http.check.ok set to 0.

### Command

Command probe executes command which outputs like Mackerel metric plugin.

```yaml
command:
  command: "/path/to/metric-command" # Path to execute command
  timeout: "5s"                      # Seconds of command timeout (default 15)
```

Command probe handles command's output as host metric.

#### Example of automated cleanup for terminated EC2 instances.

Command probe can run any scripts against for Mackerel hosts.

For example,

```yaml
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
aws ec2 describe-instance-status --instance-id "${instance_id}" || mkr retire --force "${host_id}"
### XXX must retry...
```

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
