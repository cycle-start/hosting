//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// findAllContainersByEnv finds all running containers that have the given
// environment variable set to the given value.
func findAllContainersByEnv(ctx context.Context, t *testing.T, key, value string) []string {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("create docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}

	target := key + "=" + value
	var ids []string
	for _, c := range containers {
		info, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}
		for _, env := range info.Config.Env {
			if env == target {
				ids = append(ids, c.ID)
				break
			}
		}
	}

	if len(ids) == 0 {
		t.Fatalf("no running container with %s=%s", key, value)
	}
	return ids
}

// findContainerByEnv finds the first running container that has the given
// environment variable set to the given value.
func findContainerByEnv(ctx context.Context, t *testing.T, key, value string) string {
	t.Helper()
	ids := findAllContainersByEnv(ctx, t, key, value)
	return ids[0]
}

// execInContainer runs a command inside a container and returns stdout.
func execInContainer(ctx context.Context, t *testing.T, containerID string, cmd []string) string {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("create docker client: %v", err)
	}
	defer cli.Close()

	execID, err := cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		t.Fatalf("exec create: %v", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		t.Fatalf("exec attach: %v", err)
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		t.Fatalf("exec read output: %v", err)
	}

	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		t.Fatalf("exec inspect: %v", err)
	}
	if inspectResp.ExitCode != 0 {
		t.Fatalf("exec exited %d: stdout=%s stderr=%s", inspectResp.ExitCode, stdout.String(), stderr.String())
	}

	return stdout.String()
}

// noRedirectClient is an HTTP client that does not follow redirects.
var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// httpGetWithHost performs an HTTP GET with a custom Host header.
// It does not follow redirects to avoid DNS resolution of virtual hostnames.
func httpGetWithHost(url, host string) (*http.Response, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	if host != "" {
		req.Host = host
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, "", fmt.Errorf("read body: %w", err)
	}

	return resp, string(body), nil
}

// waitForHTTP retries an HTTP GET with the given Host header until it
// succeeds (2xx) or the timeout elapses.
func waitForHTTP(t *testing.T, url, host string, timeout time.Duration) (*http.Response, string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		resp, body, err := httpGetWithHost(url, host)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, body
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(body))
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("timed out waiting for %s (Host: %s): %v", url, host, lastErr)
	return nil, ""
}
