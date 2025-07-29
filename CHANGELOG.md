# Changelog

## [v0.8.1](https://github.com/fujiwara/maprobe/compare/v0.8.0...v0.8.1) - 2025-07-29
- Do not overwrite service.name by @fujiwara in https://github.com/fujiwara/maprobe/pull/118

## [v0.8.0](https://github.com/fujiwara/maprobe/compare/v0.7.7...v0.8.0) - 2025-07-29
- Bump github.com/fujiwara/ridge from 0.6.2 to 0.13.0 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/93
- modernization by @fujiwara in https://github.com/fujiwara/maprobe/pull/98
- replace kingpin with kong for CLI parsing by @fujiwara in https://github.com/fujiwara/maprobe/pull/99
- Add slog support by @fujiwara in https://github.com/fujiwara/maprobe/pull/101
- Migrate aws sdk go v2 by @fujiwara in https://github.com/fujiwara/maprobe/pull/102
- Add structured logging with slog and OpenTelemetry metrics integration by @fujiwara in https://github.com/fujiwara/maprobe/pull/103
- Add host.id to otel metrics if not empty. by @fujiwara in https://github.com/fujiwara/maprobe/pull/108
- Add certificate expiration metrics by @fujiwara in https://github.com/fujiwara/maprobe/pull/109
- Update Go toolchain to go1.24.4 by @github-actions[bot] in https://github.com/fujiwara/maprobe/pull/110
- Update Go toolchain to go1.24.5 by @github-actions[bot] in https://github.com/fujiwara/maprobe/pull/114
- Release for v0.8.0 by @github-actions[bot] in https://github.com/fujiwara/maprobe/pull/100

## [v0.8.0](https://github.com/fujiwara/maprobe/compare/v0.7.7...v0.8.0) - 2025-07-27
- Bump github.com/fujiwara/ridge from 0.6.2 to 0.13.0 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/93
- modernization by @fujiwara in https://github.com/fujiwara/maprobe/pull/98
- replace kingpin with kong for CLI parsing by @fujiwara in https://github.com/fujiwara/maprobe/pull/99
- Add slog support by @fujiwara in https://github.com/fujiwara/maprobe/pull/101
- Migrate aws sdk go v2 by @fujiwara in https://github.com/fujiwara/maprobe/pull/102
- Add structured logging with slog and OpenTelemetry metrics integration by @fujiwara in https://github.com/fujiwara/maprobe/pull/103
- Add host.id to otel metrics if not empty. by @fujiwara in https://github.com/fujiwara/maprobe/pull/108
- Add certificate expiration metrics by @fujiwara in https://github.com/fujiwara/maprobe/pull/109
- Update Go toolchain to go1.24.4 by @github-actions[bot] in https://github.com/fujiwara/maprobe/pull/110
- Update Go toolchain to go1.24.5 by @github-actions[bot] in https://github.com/fujiwara/maprobe/pull/114

## [v0.7.7](https://github.com/fujiwara/maprobe/compare/v0.7.6...v0.7.7) - 2025-05-21
- use the latest patch version of go by @fujiwara in https://github.com/fujiwara/maprobe/pull/97

## [v0.7.6](https://github.com/fujiwara/maprobe/compare/v0.7.5...v0.7.6) - 2025-05-21
- update to go1.24.3 by @fujiwara in https://github.com/fujiwara/maprobe/pull/95
- Bump golang.org/x/net from 0.36.0 to 0.38.0 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/96

## [v0.7.5](https://github.com/fujiwara/maprobe/compare/v0.7.4...v0.7.5) - 2025-03-12
- supports debian:bookworm by @fujiwara in https://github.com/fujiwara/maprobe/pull/85
- Bump golang.org/x/net from 0.23.0 to 0.36.0 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/86

## [v0.7.4](https://github.com/fujiwara/maprobe/compare/v0.7.3...v0.7.4) - 2024-05-14
- embed Version by make install by @fujiwara in https://github.com/fujiwara/maprobe/pull/59
- Retry for posting metrics by @fujiwara in https://github.com/fujiwara/maprobe/pull/61

## [v0.7.3](https://github.com/fujiwara/maprobe/compare/v0.7.2...v0.7.3) - 2024-05-13
- The version was fixed at 0.5.4, so I changed it to embed it at build time. by @mashiike in https://github.com/fujiwara/maprobe/pull/52
- Bump google.golang.org/grpc from 1.57.0 to 1.57.1 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/43
- update golang.org/x/net@v0.23.0 by @fujiwara in https://github.com/fujiwara/maprobe/pull/53
- Bump google.golang.org/protobuf from 1.31.0 to 1.33.0 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/44
- Bump actions/setup-go from 4 to 5 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/58
- Bump docker/login-action from 2 to 3 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/57
- Bump docker/setup-qemu-action from 2 to 3 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/56
- Bump docker/setup-buildx-action from 2 to 3 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/55

## [v0.7.2](https://github.com/fujiwara/maprobe/compare/v0.7.1...v0.7.2) - 2024-04-05
- metric name is required. by @fujiwara in https://github.com/fujiwara/maprobe/pull/45

## [v0.7.1](https://github.com/fujiwara/maprobe/compare/v0.7.0...v0.7.1) - 2024-01-19
- Fix AggregatedMetrics not being posted to Mackerel by @cohalz in https://github.com/fujiwara/maprobe/pull/34

## [v0.7.0](https://github.com/fujiwara/maprobe/compare/v0.6.2...v0.7.0) - 2023-12-18
- Otel metric by @fujiwara in https://github.com/fujiwara/maprobe/pull/29

## [v0.6.4](https://github.com/fujiwara/maprobe/compare/v0.6.3...v0.6.4) - 2023-12-18

## [v0.6.3](https://github.com/fujiwara/maprobe/compare/v0.6.2...v0.6.3) - 2023-12-18

## [v0.6.2](https://github.com/fujiwara/maprobe/compare/v0.6.1...v0.6.2) - 2023-10-25
- Graceful shutdown sub processes. by @fujiwara in https://github.com/fujiwara/maprobe/pull/32

## [v0.6.1](https://github.com/fujiwara/maprobe/compare/v0.6.0...v0.6.1) - 2023-10-17
- Backquotes were missing on README.md by @do-su-0805 in https://github.com/fujiwara/maprobe/pull/28
- Bump golang.org/x/net from 0.0.0-20220127200216-cd36cc0744dd to 0.17.0 by @dependabot[bot] in https://github.com/fujiwara/maprobe/pull/30
- update to fujiwara/ridge v0.6.2 by @fujiwara in https://github.com/fujiwara/maprobe/pull/31

## [v0.6.0](https://github.com/fujiwara/maprobe/compare/v0.5.4...v0.6.0) - 2023-06-19
- Feature/probe service metrics by @fujiwara in https://github.com/fujiwara/maprobe/pull/27

## [v0.5.4](https://github.com/fujiwara/maprobe/compare/v0.5.3...v0.5.4) - 2023-06-07

## [v0.5.3](https://github.com/fujiwara/maprobe/compare/v0.5.2...v0.5.3) - 2023-06-07

## [v0.5.2](https://github.com/fujiwara/maprobe/compare/v0.5.1...v0.5.2) - 2023-06-07
- add workflow to build and publish arm64 container image and binary by @Azuki-bar in https://github.com/fujiwara/maprobe/pull/26

## [v0.5.1](https://github.com/fujiwara/maprobe/compare/v0.4.5...v0.5.1) - 2023-03-29
- run on AWS Lambda. by @fujiwara in https://github.com/fujiwara/maprobe/pull/24
- add -gops option (default false) by @fujiwara in https://github.com/fujiwara/maprobe/pull/25

## [v0.5.0](https://github.com/fujiwara/maprobe/compare/v0.4.4...v0.5.0) - 2022-07-14
- update modules by @fujiwara in https://github.com/fujiwara/maprobe/pull/23

## [v0.4.5](https://github.com/fujiwara/maprobe/compare/v0.4.4...v0.4.5) - 2022-07-14
- update modules by @fujiwara in https://github.com/fujiwara/maprobe/pull/23

## [v0.4.4](https://github.com/fujiwara/maprobe/compare/v0.4.3...v0.4.4) - 2022-07-14
- bump go1.18.4 by @fujiwara in https://github.com/fujiwara/maprobe/pull/22

## [v0.4.3](https://github.com/fujiwara/maprobe/compare/v0.4.2...v0.4.3) - 2021-12-20
