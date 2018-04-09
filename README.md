# maprobe

Mackerel external probe agent.

## Description

maprobe is a external probe agent with [Mackerel](https://mackerel.io).

maprobe works as below.

1. Fetch hosts information from Mackerel API.
  - Specified service and role.
1. Execute probes (ping, tcp, http, command) to the hosts.
  - expand place holder `{{ .Host }}` as [Mackerel host struct](https://godoc.org/github.com/mackerelio/mackerel-client-go#Host).
  - `{{ .Host.IPAddress.eth0 }}` expand to e.g. `192.168.1.1`
1. Post host metrics to Mackerel.


## Configuration

```yaml
probe_only: false   # when true, do not post metrics. only dump to debug log.
probes:
  - service: production
    role: server
    ping:
      address: '{{ .Host.IPAddresses.eth0 }}'

  - service: production
    role: load_balancer
    http:
      url: 'http://{{ .Host.CustomIdentifier }}/ping'
      post: POST
      headers:
        Cache-Control: no-cache
        Content-Type: application/json
      body: '{"hello":"world"}'
      expect_pattern: '^ok'

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
