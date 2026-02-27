package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
)

type Terminal struct {
	authSvc       *core.AuthService
	customerSvc   *core.CustomerService
	hostingClient *hosting.Client
	hostingWSURL  string
	hostingAPIKey string
}

func NewTerminal(
	authSvc *core.AuthService,
	customerSvc *core.CustomerService,
	hostingClient *hosting.Client,
	hostingWSURL string,
	hostingAPIKey string,
) *Terminal {
	return &Terminal{
		authSvc:       authSvc,
		customerSvc:   customerSvc,
		hostingClient: hostingClient,
		hostingWSURL:  hostingWSURL,
		hostingAPIKey: hostingAPIKey,
	}
}

// Connect accepts a WebSocket from the browser, authenticates via JWT query param,
// verifies the user has access to the webroot's tenant, then proxies messages to
// the hosting API's terminal endpoint.
func (t *Terminal) Connect(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := t.authSvc.ValidateToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	webrootID := chi.URLParam(r, "id")
	if webrootID == "" {
		http.Error(w, "missing webroot id", http.StatusBadRequest)
		return
	}

	webroot, err := t.hostingClient.GetWebroot(r.Context(), webrootID)
	if err != nil {
		http.Error(w, "webroot not found", http.StatusNotFound)
		return
	}

	customerID, err := t.customerSvc.GetCustomerIDByTenant(r.Context(), webroot.TenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	hasAccess, err := t.customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil || !hasAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Accept the incoming WebSocket from the browser.
	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	defer clientConn.CloseNow()

	// Dial the hosting API terminal endpoint.
	hostingURL := fmt.Sprintf("%s/api/v1/tenants/%s/terminal?token=%s",
		t.hostingWSURL, webroot.TenantID, t.hostingAPIKey)

	hostingConn, _, err := websocket.Dial(r.Context(), hostingURL, nil)
	if err != nil {
		clientConn.Close(websocket.StatusInternalError, "failed to connect to terminal backend")
		return
	}
	defer hostingConn.CloseNow()

	// Proxy messages bidirectionally.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// client → hosting
	go func() {
		defer cancel()
		proxy(ctx, hostingConn, clientConn)
	}()

	// hosting → client
	proxy(ctx, clientConn, hostingConn)
}

// proxy copies WebSocket messages from src to dst, preserving message type.
func proxy(ctx context.Context, dst, src *websocket.Conn) {
	for {
		msgType, data, err := src.Read(ctx)
		if err != nil {
			return
		}
		if err := dst.Write(ctx, msgType, data); err != nil {
			return
		}
	}
}
