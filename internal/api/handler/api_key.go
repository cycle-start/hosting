package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/go-chi/chi/v5"
)

// APIKey handles API key management endpoints.
type APIKey struct {
	svc *core.APIKeyService
}

// NewAPIKey creates a new APIKey handler.
func NewAPIKey(svc *core.APIKeyService) *APIKey {
	return &APIKey{svc: svc}
}

// Create generates a new API key. The raw key is returned once in the response.
func (h *APIKey) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateAPIKey
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key, rawKey, err := h.svc.Create(r.Context(), req.Name, req.Scopes)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build the response with the raw key included (shown only once).
	resp := map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"key":        rawKey,
		"key_prefix": key.KeyPrefix,
		"scopes":     key.Scopes,
		"created_at": key.CreatedAt,
	}
	response.WriteJSON(w, http.StatusCreated, resp)
}

// List lists all API keys with cursor-based pagination.
func (h *APIKey) List(w http.ResponseWriter, r *http.Request) {
	pg := request.ParsePagination(r)

	keys, hasMore, err := h.svc.List(r.Context(), pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(keys) > 0 {
		nextCursor = keys[len(keys)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, keys, nextCursor, hasMore)
}

// Get retrieves an API key by ID.
func (h *APIKey) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, key)
}

// Revoke soft-deletes an API key by setting revoked_at.
func (h *APIKey) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Revoke(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
