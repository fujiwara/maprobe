# Claude Code Instructions for maprobe

This document contains instructions and context for Claude Code when working on the maprobe project.

## Project Overview

maprobe is a monitoring probe tool written in Go that supports various probe types (HTTP, TCP, ping) and can send metrics to Mackerel and OpenTelemetry endpoints.

## Development Guidelines

### Code Style
- Follow Go conventions and existing code patterns
- Use existing libraries and utilities already present in the codebase
- Do not add comments unless explicitly requested
- Maintain backward compatibility when making changes

### Git Workflow
- Always work on feature branches, never commit directly to main
- Use descriptive branch names (e.g., `add-slog-support`, `fix-http-parsing`)
- Only stage and commit files that were actually modified
- Use `git add <specific-files>` instead of `git add .`
- Create branches with `git switch -c <branch-name>`

### Testing
- Always run tests with `make test` before committing
- Ensure builds pass with `go build ./cmd/maprobe`
- Test CLI functionality manually when modifying command-line interfaces

### Logging
- The project now uses structured logging with slog
- Use appropriate log levels: Debug, Info, Warn, Error
- Use key-value pairs for structured logging: `slog.Info("message", "key", value)`
- Do not include log level prefixes in messages (slog handles this automatically)
- Support both text and JSON log formats via `--log-format` flag

### CLI Development
- The project uses Kong for CLI parsing
- Be aware that Kong may return command names in "command <arg>" format
- Extract base command names using `strings.Cut(cmdName, " ")`
- Add global flags to the CLI struct in maprobe.go
- Support environment variables for configuration options

## Build and Test Commands

```bash
# Build the project
make cmd/maprobe/maprobe
# or
go build ./cmd/maprobe

# Run tests
make test

# Clean build artifacts
make clean
```

## Recent Changes

### Structured Logging Migration
- Added `--log-format=text|json` global flag
- Migrated from standard `log` package to `log/slog`
- Implemented setupSlog() function for handler configuration
- Replaced log calls in maprobe.go and cmd/maprobe/main.go
- Other files (http.go, ping.go, tcp.go, etc.) still use old logging and need migration

### Kong CLI Migration
- Migrated from kingpin to Kong for CLI parsing
- Fixed command parsing for commands with arguments
- Added backward compatibility aliases for CLI flags

## File Structure

- `cmd/maprobe/main.go` - Main entry point and CLI setup
- `maprobe.go` - Core functionality and CLI struct definitions
- `http.go`, `tcp.go`, `ping.go` - Individual probe implementations
- `config.go` - Configuration loading and validation
- `client.go` - Mackerel API client
- `probe.go` - Probe execution logic

## Common Tasks

### Adding New CLI Flags
1. Add the flag to the appropriate Cmd struct in maprobe.go
2. Use Kong tags for help text, defaults, and environment variables
3. Update CLI parsing logic in main.go if needed
4. Test with `--help` to verify flag appears correctly

### Adding New Log Calls
- Use slog with appropriate levels and structured data
- Example: `slog.Info("operation completed", "duration", elapsed, "items", count)`
- Don't include level prefixes like "[info]" in messages

### Testing CLI Commands
```bash
# Test with different log formats
./cmd/maprobe/maprobe --log-format=json --log-level=debug http https://example.com
./cmd/maprobe/maprobe --log-format=text --log-level=info ping 8.8.8.8
```

