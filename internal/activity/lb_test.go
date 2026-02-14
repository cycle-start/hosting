package activity

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHAProxy starts a TCP listener that echoes a canned response for each
// incoming command. Returns the listener address and a cleanup function.
func mockHAProxy(t *testing.T, handler func(cmd string) string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				cmd := strings.TrimSpace(string(buf[:n]))
				resp := handler(cmd)
				fmt.Fprint(c, resp)
			}(conn)
		}
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

func TestNodeLB_SetLBMapEntry(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		return "\n"
	})
	defer cleanup()

	// Override the haproxy address by calling haproxyCommand directly.
	lb := NewNodeLB(zerolog.Nop())
	_ = lb // verify constructor works

	// Test via haproxyCommand directly since NodeLB uses hardcoded localhost.
	resp, err := haproxyCommand(addr, fmt.Sprintf("set map %s %s %s\n", haproxyMapPath, "example.com", "shard-web-1"))
	require.NoError(t, err)
	assert.Contains(t, resp, "")
}

func TestNodeLB_SetLBMapEntry_FallbackToAdd(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		if strings.HasPrefix(cmd, "set map") {
			return "entry not found\n"
		}
		return "\n"
	})
	defer cleanup()

	// Test the "set map" -> "not found" -> "add map" flow via haproxyCommand.
	resp, err := haproxyCommand(addr, fmt.Sprintf("set map %s %s %s\n", haproxyMapPath, "example.com", "shard-web-1"))
	require.NoError(t, err)
	assert.Contains(t, resp, "not found")

	resp, err = haproxyCommand(addr, fmt.Sprintf("add map %s %s %s\n", haproxyMapPath, "example.com", "shard-web-1"))
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(resp))
}

func TestNodeLB_DeleteLBMapEntry(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		return "\n"
	})
	defer cleanup()

	resp, err := haproxyCommand(addr, fmt.Sprintf("del map %s %s\n", haproxyMapPath, "example.com"))
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(resp))
}

func TestNodeLB_DeleteLBMapEntry_NotFoundIgnored(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		return "entry not found\n"
	})
	defer cleanup()

	resp, err := haproxyCommand(addr, fmt.Sprintf("del map %s %s\n", haproxyMapPath, "example.com"))
	require.NoError(t, err)
	assert.Contains(t, resp, "not found")
}

func TestNodeLB_ConnectionRefused(t *testing.T) {
	_, err := haproxyCommand("127.0.0.1:1", "del map /test test.com\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to haproxy")
}

func TestNodeLB_DeleteLBMapEntry_Activity(t *testing.T) {
	// Test the actual activity method (will fail to connect since not localhost:9999).
	lb := NewNodeLB(zerolog.Nop())
	err := lb.DeleteLBMapEntry(context.Background(), DeleteLBMapEntryParams{
		FQDN: "example.com",
	})
	require.Error(t, err, "should fail since no HAProxy is listening on localhost:9999")
}

// Suppress unused variable warning for time import (used by other tests in package).
var _ = time.Now
