package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/sshca"
)

type Terminal struct {
	ca *sshca.CA
	db *pgxpool.Pool
}

func NewTerminal(ca *sshca.CA, db *pgxpool.Pool) *Terminal {
	return &Terminal{ca: ca, db: db}
}

// resizeMsg is a control message sent by the client to resize the terminal.
type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// Connect upgrades to WebSocket and proxies an SSH session to the tenant's web node.
func (h *Terminal) Connect(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing tenant ID")
		return
	}

	// Auth via query param (WebSocket API doesn't support custom headers).
	token := r.URL.Query().Get("token")
	if token == "" {
		response.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	if err := h.validateToken(r.Context(), token); err != nil {
		response.WriteError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	// Look up tenant.
	var sshEnabled bool
	var shardID *string
	err := h.db.QueryRow(r.Context(),
		`SELECT ssh_enabled, shard_id FROM tenants WHERE id = $1`, tenantID,
	).Scan(&sshEnabled, &shardID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if !sshEnabled {
		response.WriteError(w, http.StatusForbidden, "SSH is not enabled for this tenant")
		return
	}
	if shardID == nil {
		response.WriteError(w, http.StatusConflict, "tenant has no shard assigned")
		return
	}

	// Find a web node for this shard.
	nodeIP, err := h.resolveNodeIP(r.Context(), *shardID)
	if err != nil {
		log.Error().Err(err).Str("shard_id", *shardID).Msg("failed to resolve web node IP")
		response.WriteError(w, http.StatusServiceUnavailable, "no available web node")
		return
	}

	// Sign ephemeral certificate.
	certSigner, err := h.ca.Sign(tenantID, 60*time.Second)
	if err != nil {
		log.Error().Err(err).Str("tenant", tenantID).Msg("failed to sign SSH certificate")
		response.WriteError(w, http.StatusInternalServerError, "failed to create SSH credentials")
		return
	}

	// Upgrade to WebSocket.
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Origin differs from Host when proxied through admin-ui.
	})
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}
	defer ws.CloseNow()

	// Dial SSH via TCP.
	addr := nodeIP + ":22"
	tcpConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		log.Error().Err(err).Str("addr", addr).Msg("TCP dial to SSH node failed")
		ws.Close(websocket.StatusInternalError, fmt.Sprintf("SSH connection failed: %s", err))
		return
	}
	defer tcpConn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, &ssh.ClientConfig{
		User:            tenantID,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(certSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		log.Error().Err(err).Str("node_ip", nodeIP).Str("tenant", tenantID).Msg("SSH handshake failed")
		ws.Close(websocket.StatusInternalError, fmt.Sprintf("SSH connection failed: %s", err))
		return
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		log.Error().Err(err).Msg("SSH session creation failed")
		ws.Close(websocket.StatusInternalError, "SSH session failed")
		return
	}
	defer session.Close()

	// Request PTY.
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		log.Error().Err(err).Msg("PTY request failed")
		ws.Close(websocket.StatusInternalError, "PTY request failed")
		return
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		ws.Close(websocket.StatusInternalError, "stdin pipe failed")
		return
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		ws.Close(websocket.StatusInternalError, "stdout pipe failed")
		return
	}

	if err := session.Shell(); err != nil {
		log.Error().Err(err).Msg("shell start failed")
		ws.Close(websocket.StatusInternalError, "shell start failed")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// SSH stdout -> WebSocket (binary).
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				if writeErr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); writeErr != nil {
					cancel()
					return
				}
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// WebSocket -> SSH stdin. Text messages are control messages (resize).
	for {
		msgType, data, err := ws.Read(ctx)
		if err != nil {
			break
		}

		switch msgType {
		case websocket.MessageBinary:
			if _, err := stdinPipe.Write(data); err != nil {
				cancel()
				return
			}
		case websocket.MessageText:
			var msg resizeMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
				_ = session.WindowChange(msg.Rows, msg.Cols)
			}
		}
	}

	ws.Close(websocket.StatusNormalClosure, "")
}

// validateToken checks the API key against the database (same logic as auth middleware).
func (h *Terminal) validateToken(ctx context.Context, key string) error {
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])
	var id string
	return h.db.QueryRow(ctx,
		`SELECT id FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL`,
		keyHash,
	).Scan(&id)
}

// resolveNodeIP finds the IP of an active node in the given shard.
func (h *Terminal) resolveNodeIP(ctx context.Context, shardID string) (string, error) {
	var ip string
	err := h.db.QueryRow(ctx, `
		SELECT host(n.ip_address) FROM nodes n
		JOIN node_shard_assignments nsa ON nsa.node_id = n.id
		WHERE nsa.shard_id = $1 AND n.status = 'active' AND n.ip_address IS NOT NULL
		ORDER BY n.id
		LIMIT 1
	`, shardID).Scan(&ip)
	if err != nil {
		return "", fmt.Errorf("resolve node IP for shard %s: %w", shardID, err)
	}
	// Strip CIDR suffix — Postgres INET returns e.g. "10.10.10.10/32".
	if idx := strings.IndexByte(ip, '/'); idx != -1 {
		ip = ip[:idx]
	}
	return ip, nil
}
