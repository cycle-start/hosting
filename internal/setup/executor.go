package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/creack/pty"
)

// StepID identifies a deployment step.
type StepID string

const (
	StepSSHCA       StepID = "ssh_ca"
	StepAnsible     StepID = "ansible"
	StepRegisterKey StepID = "register_api_key"
	StepClusterApply StepID = "cluster_apply"
	StepSeed        StepID = "seed"
)

// StepDef describes a deployment step for the UI.
type StepDef struct {
	ID          StepID `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	MultiOnly   bool   `json:"multi_only"`
}

// AllSteps returns the ordered list of deployment steps.
func AllSteps() []StepDef {
	return []StepDef{
		{StepSSHCA, "Generate SSH CA keypair", "Generate an ed25519 SSH Certificate Authority keypair for node-to-node communication.", true},
		{StepAnsible, "Run Ansible provisioning", "Install packages, configure services, and deploy agents on all hosts.", false},
		{StepRegisterKey, "Register API key", "Create the authentication key used by all components to communicate with the control plane.", false},
		{StepClusterApply, "Register cluster topology", "Tell the control plane about the region, cluster, nodes, shards, and runtimes.", false},
		{StepSeed, "Seed initial brand", "Create the first brand with its domains, nameservers, and mail configuration.", false},
	}
}

// validStepIDs returns the set of valid step IDs for quick lookup.
func validStepIDs() map[StepID]bool {
	m := make(map[StepID]bool)
	for _, s := range AllSteps() {
		m[s.ID] = true
	}
	return m
}

// ExecEvent is a single NDJSON event streamed to the client.
type ExecEvent struct {
	Type     string `json:"type"`               // "output", "done", "error"
	Data     string `json:"data,omitempty"`      // line text or error message
	Stream   string `json:"stream,omitempty"`    // "stdout" or "stderr"
	ExitCode *int   `json:"exit_code,omitempty"` // only for "done"
}

// stepCommand returns the working directory, command name, and arguments for a step.
func stepCommand(id StepID, cfg *Config, outputDir string) (dir string, name string, args []string, err error) {
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve output dir: %w", err)
	}

	switch id {
	case StepSSHCA:
		return ".", "ssh-keygen", []string{"-t", "ed25519", "-f", "ssh_ca", "-N", ""}, nil
	case StepAnsible:
		inventory := filepath.Join(absOutput, "generated", "ansible", "inventory", "static.ini")
		return "ansible", "ansible-playbook", []string{"site.yml", "-i", inventory}, nil
	case StepRegisterKey:
		return ".", "bin/core-api", []string{"create-api-key", "--name", "setup", "--raw-key", cfg.APIKey}, nil
	case StepClusterApply:
		clusterFile := filepath.Join(absOutput, "generated", "cluster.yaml")
		return ".", "bin/hostctl", []string{"cluster", "apply", "-f", clusterFile}, nil
	case StepSeed:
		seedFile := filepath.Join(absOutput, "generated", "seed.yaml")
		return ".", "bin/hostctl", []string{"seed", "-f", seedFile}, nil
	default:
		return "", "", nil, fmt.Errorf("unknown step: %s", id)
	}
}

// writeEvent writes a single NDJSON event to the response.
func writeEvent(w http.ResponseWriter, flusher http.Flusher, event ExecEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	w.Write(data)
	w.Write([]byte("\n"))
	flusher.Flush()
}

// executeStep runs a deployment step and streams output as NDJSON.
// It uses a PTY so that child processes detect a terminal and emit colors.
func (s *Server) executeStep(w http.ResponseWriter, r *http.Request, flusher http.Flusher, stepID StepID) {
	s.mu.Lock()
	cfg := *s.config // copy
	outputDir := s.outputDir
	s.mu.Unlock()

	dir, name, args, err := stepCommand(stepID, &cfg, outputDir)
	if err != nil {
		writeEvent(w, flusher, ExecEvent{Type: "error", Data: err.Error()})
		return
	}

	ctx := r.Context()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		writeEvent(w, flusher, ExecEvent{Type: "error", Data: "start: " + err.Error()})
		return
	}
	defer ptmx.Close()

	// Stream PTY output line by line.
	scanner := bufio.NewScanner(ptmx)
	for scanner.Scan() {
		writeEvent(w, flusher, ExecEvent{Type: "output", Data: scanner.Text(), Stream: "stdout"})
	}
	// Ignore EIO — expected when PTY child exits.
	if err := scanner.Err(); err != nil && err != io.EOF {
		// EIO is normal on PTY close, don't treat it as an error.
	}

	err = cmd.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			writeEvent(w, flusher, ExecEvent{Type: "error", Data: err.Error()})
			return
		}
	}

	writeEvent(w, flusher, ExecEvent{Type: "done", ExitCode: &exitCode})
}
