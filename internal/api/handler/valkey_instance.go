package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type ValkeyInstance struct {
	svc *core.ValkeyInstanceService
}

func NewValkeyInstance(svc *core.ValkeyInstanceService) *ValkeyInstance {
	return &ValkeyInstance{svc: svc}
}

func (h *ValkeyInstance) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	instances, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range instances {
		instances[i].Password = ""
	}
	var nextCursor string
	if hasMore && len(instances) > 0 {
		nextCursor = instances[len(instances)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, instances, nextCursor, hasMore)
}

func (h *ValkeyInstance) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateValkeyInstance
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	maxMemoryMB := req.MaxMemoryMB
	if maxMemoryMB == 0 {
		maxMemoryMB = 64
	}

	now := time.Now()
	shardID := req.ShardID
	instance := &model.ValkeyInstance{
		ID:          platform.NewID(),
		TenantID:    &tenantID,
		Name:        req.Name,
		ShardID:     &shardID,
		MaxMemoryMB: maxMemoryMB,
		Password:    generatePassword(),
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), instance); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	instance.Password = ""
	response.WriteJSON(w, http.StatusAccepted, instance)
}

func (h *ValkeyInstance) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	instance.Password = ""
	response.WriteJSON(w, http.StatusOK, instance)
}

func (h *ValkeyInstance) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *ValkeyInstance) Migrate(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.MigrateValkeyInstance
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Migrate(r.Context(), id, req.TargetShardID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *ValkeyInstance) ReassignTenant(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.ReassignValkeyInstanceTenant
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.ReassignTenant(r.Context(), id, req.TenantID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	instance, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	instance.Password = ""
	response.WriteJSON(w, http.StatusOK, instance)
}

// generatePassword creates a random 32-character hex password.
func generatePassword() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
