package maprobe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
)

const CustomPrefix = "custom."

var DefaultCommandTimeout = 15 * time.Second

var graphDefsPosted = sync.Map{}

type CommandProbeConfig struct {
	RawCommand interface{}   `yaml:"command"`
	command    []string      `yaml:"-"`
	Timeout    time.Duration `yaml:"timeout"`
	GraphDefs  bool          `yaml:"graph_defs"`
}

func (pc *CommandProbeConfig) initialize() error {
	switch c := pc.RawCommand.(type) {
	case []interface{}:
		if len(c) == 0 {
			return errors.Errorf("command is empty array")
		}
		for _, v := range c {
			switch s := v.(type) {
			case string:
				pc.command = append(pc.command, s)
			default:
				return errors.Errorf("command must be array of string")
			}
		}
	case string:
		if len(c) == 0 {
			return errors.Errorf("command is empty string")
		}
		pc.command = []string{c}
	case nil:
		return errors.Errorf("command is empty")
	default:
		return errors.Errorf("invalid command: %#v", pc.RawCommand)
	}
	return nil
}

func (pc *CommandProbeConfig) GenerateProbe(host *mackerel.Host, client *mackerel.Client) (Probe, error) {
	p := &CommandProbe{
		hostID:    host.ID,
		Timeout:   pc.Timeout,
		GraphDefs: pc.GraphDefs,
		Command:   make([]string, len(pc.command)),
	}
	var err error

	for i, c := range pc.command {
		p.Command[i], err = expandPlaceHolder(c, host)
		if err != nil {
			return nil, errors.Wrap(err, "invalid command")
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
			log.Printf("[warn] failed to post graph defs for %#v: %s", p, err)
		}
	}

	return p, nil
}

type CommandProbe struct {
	hostID string

	Command   []string
	Timeout   time.Duration
	GraphDefs bool
}

func (p *CommandProbe) HostID() string {
	return p.hostID
}

func (p *CommandProbe) MetricName(name string) string {
	return name
}

func (p *CommandProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *CommandProbe) Run(ctx context.Context) (ms HostMetrics, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch len(p.Command) {
	case 0:
		return nil, errors.New("no command")
	case 1:
		cmd = exec.CommandContext(ctx, p.Command[0])
	default:
		cmd = exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
	}
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ms, errors.Wrap(err, "stdout open failed")
	}
	scanner := bufio.NewScanner(stdout)

	if err := cmd.Start(); err != nil {
		return ms, errors.Wrap(err, "command execute failed")
	}

	for scanner.Scan() {
		log.Println("[trace]", scanner.Text())
		m, err := parseMetricLine(scanner.Text())
		if err != nil {
			log.Println("[warn]", err)
			continue
		}
		m.HostID = p.hostID
		if p.GraphDefs {
			m.Name = CustomPrefix + m.Name
		}
		ms = append(ms, m)
	}

	err = cmd.Wait()
	if e, ok := err.(*exec.ExitError); ok {
		return ms, errors.Wrap(e, "command execute failed")
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
		log.Printf("[trace] graphDefsPosted %v", pc)
		return nil
	}

	out, err := p.GetGraphDefs()
	if err != nil {
		graphDefsPosted.Store(pc, struct{}{})
		return err
	}
	log.Printf("[trace] Got graph defs %#v", out)

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
	log.Println("[trace] create to graph defs", string(b))
	if err := client.CreateGraphDefs(payloads); err != nil {
		// When failed to post to Mackerel, graphDefsPosted shouldnot be stored.
		return errors.Wrap(err, "could not create graph defs")
	}
	log.Printf("[info] success to create graph defs for %s %v", p.hostID, p.Command)

	graphDefsPosted.Store(pc, struct{}{})
	return nil
}

func (p *CommandProbe) GetGraphDefs() (*GraphsOutput, error) {
	log.Printf("[trace] Get graph defs for %s %v", p.hostID, p.Command)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
	cmd.Env = append(os.Environ(), "MACKEREL_AGENT_PLUGIN_META=1")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "stdout open failed")
	}
	r := bufio.NewReader(stdout)
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "command execute failed")
	}

	header, err := r.ReadBytes('\n')
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch a first line")
	}
	if !bytes.Equal(header, pluginMetaHeaderLine) {
		// invalid header
		return nil, fmt.Errorf("%s didn't output graph defs", p.Command[0])
	}

	var out GraphsOutput
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, errors.Wrap(err, "could not decode graph defs output")
	}
	return &out, nil
}

func parseMetricLine(b string) (HostMetric, error) {
	cols := strings.SplitN(b, "\t", 3)
	if len(cols) < 3 {
		return HostMetric{}, errors.New("invalid metric format. insufficient columns")
	}
	name, value, timestamp := cols[0], cols[1], cols[2]
	m := HostMetric{
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
