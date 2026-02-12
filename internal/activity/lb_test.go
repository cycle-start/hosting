package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

// setupLBResolve sets up the mockDB to return a cluster config with the given
// haproxy_admin_addr for resolveHAProxyAddr calls.
func setupLBResolve(db *mockDB, addr string) {
	cfg := json.RawMessage(fmt.Sprintf(`{"haproxy_admin_addr":"%s"}`, addr))

	db.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "clusters")
	}), mock.Anything).Return(&lbMockRow{scanFn: func(dest ...any) error {
		*(dest[0].(*json.RawMessage)) = cfg
		return nil
	}})
}

type lbMockRow struct {
	scanFn func(dest ...any) error
}

func (r *lbMockRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

func TestLB_SetLBMapEntry(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		return "\n"
	})
	defer cleanup()

	db := &mockDB{}
	setupLBResolve(db, addr)
	lb := NewLB(db)

	err := lb.SetLBMapEntry(context.Background(), SetLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
		LBBackend: "shard-web-1",
	})
	require.NoError(t, err)
}

func TestLB_SetLBMapEntry_FallbackToAdd(t *testing.T) {
	callCount := 0
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		callCount++
		if strings.HasPrefix(cmd, "set map") {
			return "entry not found\n"
		}
		return "\n"
	})
	defer cleanup()

	db := &mockDB{}
	setupLBResolve(db, addr)
	lb := NewLB(db)

	err := lb.SetLBMapEntry(context.Background(), SetLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
		LBBackend: "shard-web-1",
	})
	require.NoError(t, err)
}

func TestLB_DeleteLBMapEntry(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		return "\n"
	})
	defer cleanup()

	db := &mockDB{}
	setupLBResolve(db, addr)
	lb := NewLB(db)

	err := lb.DeleteLBMapEntry(context.Background(), DeleteLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
	})
	require.NoError(t, err)
}

func TestLB_DeleteLBMapEntry_NotFoundIgnored(t *testing.T) {
	addr, cleanup := mockHAProxy(t, func(cmd string) string {
		return "entry not found\n"
	})
	defer cleanup()

	db := &mockDB{}
	setupLBResolve(db, addr)
	lb := NewLB(db)

	err := lb.DeleteLBMapEntry(context.Background(), DeleteLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
	})
	require.NoError(t, err, "not found errors should be ignored")
}

func TestLB_DeleteLBMapEntry_ConnectionRefused(t *testing.T) {
	// Use an address that won't be listening.
	addr := "127.0.0.1:1"

	db := &mockDB{}
	setupLBResolve(db, addr)
	lb := NewLB(db)

	err := lb.DeleteLBMapEntry(context.Background(), DeleteLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to haproxy")
}

func TestLB_ResolveHAProxyAddr_Default(t *testing.T) {
	db := &mockDB{}
	db.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "clusters")
	}), mock.Anything).Return(&lbMockRow{scanFn: func(dest ...any) error {
		*(dest[0].(*json.RawMessage)) = json.RawMessage(`{}`)
		return nil
	}})

	lb := NewLB(db)
	addr, err := lb.resolveHAProxyAddr(context.Background(), "cluster-1")
	require.NoError(t, err)
	assert.Equal(t, defaultHAProxyAdmin, addr)
}

func TestLB_ResolveHAProxyAddr_Custom(t *testing.T) {
	db := &mockDB{}
	db.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "clusters")
	}), mock.Anything).Return(&lbMockRow{scanFn: func(dest ...any) error {
		*(dest[0].(*json.RawMessage)) = json.RawMessage(`{"haproxy_admin_addr":"10.0.0.1:9999"}`)
		return nil
	}})

	lb := NewLB(db)
	addr, err := lb.resolveHAProxyAddr(context.Background(), "cluster-1")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:9999", addr)
}

// Suppress unused variable warning for time import (used by other tests in package).
var _ = time.Now
