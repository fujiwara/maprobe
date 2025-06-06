package maprobe

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

const CustomPrefix = "custom."

var DefaultCommandTimeout = 15 * time.Second

var graphDefsPosted = sync.Map{}

type CommandProbeConfig struct {
	RawCommand interface{}       `yaml:"command"`
	command    []string          `yaml:"-"`
	Timeout    time.Duration     `yaml:"timeout"`
	GraphDefs  bool              `yaml:"graph_defs"`
	Env        map[string]string `yaml:"env"`
}

func (pc *CommandProbeConfig) initialize() error {
	switch c := pc.RawCommand.(type) {
	case []interface{}:
		if len(c) == 0 {
			return fmt.Errorf("command is empty array")
		}
		for _, v := range c {
			switch s := v.(type) {
			case string:
				pc.command = append(pc.command, s)
			default:
				return fmt.Errorf("command must be array of string")
			}
		}
	case string:
		if len(c) == 0 {
			return fmt.Errorf("command is empty string")
		}
		pc.command = []string{c}
	case nil:
		return fmt.Errorf("command is empty")
	default:
		return fmt.Errorf("invalid command: %#v", pc.RawCommand)
	}
	return nil
}

func (pc *CommandProbeConfig) GenerateProbe(host *mackerel.Host, client *mackerel.Client) (Probe, error) {
	p := &CommandProbe{
		Timeout:   pc.Timeout,
		GraphDefs: pc.GraphDefs,
		Command:   make([]string, len(pc.command)),
	}
	var err error

	for i, c := range pc.command {
		p.Command[i], err = expandPlaceHolder(c, host, pc.Env)
		if err != nil {
			return nil, fmt.Errorf("invalid command: %w", err)
		}
	}

	if len(p.Command) == 1 && strings.Contains(p.Command[0], " ") {
		p.Command = []string{"sh", "-c", p.Command[0]}
	}

	if p.Timeout == 0 {
		p.Timeout = DefaultCommandTimeout
	}

	if p.GraphDefs && client != nil {
		if err := p.PostGraphDefs(client, pc); err != nil {
			slog.Warn("failed to post graph defs", "probe", p, "error", err)
		}
	}
	for name, value := range pc.Env {
		p.env = append(p.env, name+"="+value)
	}

	return p, nil
}

type CommandProbe struct {
	env []string

	Command   []string
	Timeout   time.Duration
	GraphDefs bool
}

func (p *CommandProbe) MetricName(name string) string {
	return name
}

func (p *CommandProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *CommandProbe) TempDir() string {
	s := sha256.Sum256([]byte(p.String()))
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("maprobe_command_%x", s))
	err := os.Mkdir(dir, 0700)
	if err != nil {
		if os.IsExist(err) {
			// ok
			return dir
		}
		slog.Warn("failed to create TempDir", "dir", dir, "error", err, "fallback", os.TempDir())
		return os.TempDir()
	}
	slog.Debug("TempDir created", "dir", dir, "command", strings.Join(p.Command, " "))
	return dir
}

func (p *CommandProbe) Run(ctx context.Context) (ms Metrics, err error) {
	// Create timeout context from parent context to allow cancellation
	timeoutCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch len(p.Command) {
	case 0:
		return nil, fmt.Errorf("no command")
	case 1:
		cmd = exec.CommandContext(timeoutCtx, p.Command[0])
	default:
		cmd = exec.CommandContext(timeoutCtx, p.Command[0], p.Command[1:]...)
	}
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, p.env...)
	cmd.Env = append(cmd.Env, "TMPDIR="+p.TempDir())
	cmd.Stderr = os.Stderr
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second // SIGKILL after 5 seconds
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ms, fmt.Errorf("stdout open failed: %w", err)
	}
	scanner := bufio.NewScanner(stdout)

	if err := cmd.Start(); err != nil {
		return ms, fmt.Errorf("command execute failed. %s: %w", strings.Join(p.Command, " "), err)
	}

	for scanner.Scan() {
		slog.Debug("command output", "output", scanner.Text())
		m, err := parseMetricLine(scanner.Text())
		if err != nil {
			slog.Warn("failed to parse metric line", "command", strings.Join(p.Command, " "), "error", err)
			continue
		}
		if p.GraphDefs {
			m.Name = CustomPrefix + m.Name
		}
		ms = append(ms, m)
	}

	err = cmd.Wait()
	if e, ok := err.(*exec.ExitError); ok {
		return ms, fmt.Errorf("command execute failed: %w", e)
	}

	return ms, nil
}

type GraphsOutput struct {
	Graphs map[string]Graph `json:"graphs"`
}

type Graph struct {
	Label   string            `json:"label"`
	Unit    string            `json:"unit"`
	Metrics []GraphDefsMetric `json:"metrics"`
}

type GraphDefsMetric struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Stacked bool   `json:"stacked"`
}

var pluginMetaHeaderLine = []byte("# mackerel-agent-plugin\n")

func (p *CommandProbe) PostGraphDefs(client *mackerel.Client, pc *CommandProbeConfig) error {
	if _, found := graphDefsPosted.Load(pc); found {
		slog.Debug("graph defs already posted", "config", pc)
		return nil
	}

	out, err := p.GetGraphDefs()
	if err != nil {
		graphDefsPosted.Store(pc, struct{}{})
		return err
	}
	slog.Debug("got graph defs", "defs", out)

	payloads := make([]*mackerel.GraphDefsParam, 0, len(out.Graphs))
	for _name, g := range out.Graphs {
		name := CustomPrefix + _name
		metrics := make([]*mackerel.GraphDefsMetric, 0, len(g.Metrics))
		for _, m := range g.Metrics {
			metrics = append(metrics, &mackerel.GraphDefsMetric{
				Name:        name + "." + m.Name,
				DisplayName: m.Label,
				IsStacked:   m.Stacked,
			})
		}
		payloads = append(payloads, &mackerel.GraphDefsParam{
			Name:        name,
			DisplayName: g.Label,
			Unit:        g.Unit,
			Metrics:     metrics,
		})
	}
	b, _ := json.Marshal(payloads)
	slog.Debug("creating graph defs", "payload", string(b))
	if err := client.CreateGraphDefs(payloads); err != nil {
		// When failed to post to Mackerel, graphDefsPosted shouldnot be stored.
		return fmt.Errorf("could not create graph defs: %w", err)
	}
	slog.Info("created graph defs", "command", p.Command)

	graphDefsPosted.Store(pc, struct{}{})
	return nil
}

func (p *CommandProbe) GetGraphDefs() (*GraphsOutput, error) {
	slog.Debug("getting graph defs", "command", p.Command)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
	cmd.Env = append(os.Environ(), "MACKEREL_AGENT_PLUGIN_META=1")
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second // SIGKILL after 5 seconds
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout open failed: %w", err)
	}
	r := bufio.NewReader(stdout)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("command execute failed: %w", err)
	}

	header, err := r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("could not fetch a first line: %w", err)
	}
	if !bytes.Equal(header, pluginMetaHeaderLine) {
		// invalid header
		return nil, fmt.Errorf("%s didn't output graph defs", p.Command[0])
	}

	var out GraphsOutput
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, fmt.Errorf("could not decode graph defs output: %w", err)
	}
	return &out, nil
}

func parseMetricLine(b string) (Metric, error) {
	cols := strings.SplitN(b, "\t", 3)
	if len(cols) < 3 {
		return Metric{}, fmt.Errorf("invalid metric format. insufficient columns")
	}
	name, value, timestamp := cols[0], cols[1], cols[2]
	if name == "" {
		return Metric{}, fmt.Errorf("invalid metric format. name is empty")
	}
	m := Metric{
		Name: name,
	}
	if v, err := strconv.ParseFloat(value, 64); err != nil {
		return m, fmt.Errorf("invalid metric value: %s", value)
	} else {
		m.Value = v
	}

	if ts, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
		return m, fmt.Errorf("invalid metric time: %s", timestamp)
	} else {
		m.Timestamp = time.Unix(ts, 0)
	}

	return m, nil
}
