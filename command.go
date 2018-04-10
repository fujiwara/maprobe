package maprobe

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	shellwords "github.com/mattn/go-shellwords"
	"github.com/pkg/errors"
)

var DefaultCommandTimeout = 15 * time.Second

type CommandProbeConfig struct {
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout"`
}

func (pc *CommandProbeConfig) GenerateProbe(host *mackerel.Host) (Probe, error) {
	p := &CommandProbe{
		hostID:  host.ID,
		Timeout: pc.Timeout,
	}
	var err error

	command, err := expandPlaceHolder(pc.Command, host)
	if err != nil {
		return nil, errors.Wrap(err, "invalid command")
	}
	p.Command, err = shellwords.Parse(command)
	if err != nil {
		return nil, errors.Wrap(err, "parse command failed")
	}

	if p.Timeout == 0 {
		p.Timeout = DefaultCommandTimeout
	}

	return p, nil
}

type CommandProbe struct {
	hostID string

	Command []string
	Timeout time.Duration
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

func (p *CommandProbe) Run(ctx context.Context) (ms Metrics, err error) {
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
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
		ms = append(ms, m)
	}

	err = cmd.Wait()
	if e, ok := err.(*exec.ExitError); ok {
		return ms, errors.Wrap(e, "command execute failed")
	}

	return ms, nil
}

func parseMetricLine(b string) (Metric, error) {
	cols := strings.SplitN(b, "\t", 3)
	if len(cols) < 3 {
		return Metric{}, errors.New("invalid metric format. insufficient columns")
	}
	name, value, timestamp := cols[0], cols[1], cols[2]
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
