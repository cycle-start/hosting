package handler

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"

	"github.com/go-chi/chi/v5"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/crypto"
	"github.com/edvin/hosting/internal/workflow"
)

type OIDCLogin struct {
	oidcSvc        *core.OIDCService
	temporalClient temporalclient.Client
}

func NewOIDCLogin(oidcSvc *core.OIDCService, temporalClient temporalclient.Client) *OIDCLogin {
	return &OIDCLogin{oidcSvc: oidcSvc, temporalClient: temporalClient}
}

// CreateLoginSession godoc
//
//	@Summary		Create an OIDC login session
//	@Description	Creates a short-lived login session for a tenant. The session ID is used as the login_hint in the OIDC authorize request, allowing passwordless authentication. Sessions expire after 5 minutes and can only be used once. This is how the hosting platform initiates OIDC-based login on behalf of a tenant.
//	@Tags			OIDC
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		201 {object} map[string]any
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/login-sessions [post]
func (h *OIDCLogin) CreateLoginSession(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.oidcSvc.EnsureSigningKey(r.Context()); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var databaseID *string
	if dbID := r.URL.Query().Get("database_id"); dbID != "" {
		databaseID = &dbID
	}

	session, err := h.oidcSvc.CreateLoginSession(r.Context(), id, databaseID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]any{
		"session_id": session.ID,
		"expires_at": session.ExpiresAt,
	})
}

// ValidateLoginSession godoc
//
//	@Summary		Validate a login session
//	@Description	Validates and consumes a login session. Returns the associated tenant ID if the session is valid, not expired, and not already used. Internal endpoint used by the dbadmin proxy for server-to-server authentication.
//	@Tags			Internal
//	@Security		ApiKeyAuth
//	@Param			session_id query string true "Login session ID"
//	@Success		200 {object} map[string]any
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		403 {object} response.ErrorResponse
//	@Router			/internal/v1/login-sessions/validate [post]
func (h *OIDCLogin) ValidateLoginSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing session_id parameter")
		return
	}

	session, err := h.oidcSvc.ValidateLoginSession(r.Context(), sessionID)
	if err != nil {
		response.WriteError(w, http.StatusForbidden, err.Error())
		return
	}

	result := map[string]any{
		"tenant_id": session.TenantID,
	}

	// If a database_id is attached, look up connection info (verified against session's tenant).
	if session.DatabaseID != nil {
		dbInfo, err := h.oidcSvc.GetDatabaseConnectionInfo(r.Context(), *session.DatabaseID, session.TenantID)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "failed to look up database connection info")
			return
		}
		result["database"] = dbInfo
	}

	response.WriteJSON(w, http.StatusOK, result)
}

// CreateTempAccess godoc
//
//	@Summary		Create temporary MySQL access
//	@Description	Creates a temporary MySQL user with access to a specific database. The user auto-expires after 2 hours. Internal endpoint used by the dbadmin proxy.
//	@Tags			Internal
//	@Security		ApiKeyAuth
//	@Param			id path string true "Database ID"
//	@Success		200 {object} map[string]any
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/internal/v1/databases/{id}/temp-access [post]
func (h *OIDCLogin) CreateTempAccess(w http.ResponseWriter, r *http.Request) {
	dbID, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Look up database connection info (includes primary node IP).
	// No tenant filter here — this is an internal endpoint called by dbadmin-proxy
	// after the session has already been validated with tenant ownership checks.
	dbInfo, err := h.oidcSvc.GetDatabaseConnectionInfo(r.Context(), dbID, "")
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	// Look up shard ID for the database.
	shardID, err := h.oidcSvc.GetDatabaseShardID(r.Context(), dbID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to look up database shard")
		return
	}

	// Generate temporary credentials.
	username := "tmp_" + randomAlphanumeric(8)
	password := randomAlphanumeric(32)
	passwordHash := crypto.MysqlNativePasswordHash(password)

	// Start workflow and wait for completion.
	workflowID := fmt.Sprintf("temp-access-%s-%s", dbID, username)
	run, err := h.temporalClient.ExecuteWorkflow(r.Context(), temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, workflow.CreateTempMySQLAccessWorkflow, workflow.CreateTempMySQLAccessArgs{
		DatabaseName: dbInfo.DatabaseName,
		ShardID:      shardID,
		Username:     username,
		PasswordHash: passwordHash,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to start temp access workflow")
		return
	}

	if err := run.Get(r.Context(), nil); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "temp access workflow failed: "+err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"username":      username,
		"password":      password,
		"host":          dbInfo.Host,
		"port":          dbInfo.Port,
		"database_name": dbInfo.DatabaseName,
	})
}

// randomAlphanumeric generates a random alphanumeric string of the given length.
func randomAlphanumeric(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[idx.Int64()]
	}
	return string(b)
}
