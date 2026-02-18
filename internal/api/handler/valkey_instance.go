package handler

import (
	"fmt"
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
	svc       *core.ValkeyInstanceService
	userSvc   *core.ValkeyUserService
	tenantSvc *core.TenantService
}

func NewValkeyInstance(svc *core.ValkeyInstanceService, userSvc *core.ValkeyUserService, tenantSvc *core.TenantService) *ValkeyInstance {
	return &ValkeyInstance{svc: svc, userSvc: userSvc, tenantSvc: tenantSvc}
}

// ListByTenant godoc
//
//	@Summary		List Valkey instances for a tenant
//	@Description	Returns a paginated list of Valkey (managed Redis) instances for a tenant. Passwords are redacted in list responses.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			tenantID	path		string	true	"Tenant ID"
//	@Param			limit		query		int		false	"Page size"	default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.ValkeyInstance}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/tenants/{tenantID}/valkey-instances [get]
func (h *ValkeyInstance) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	pg := request.ParsePagination(r)

	instances, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
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

// Create godoc
//
//	@Summary		Create a Valkey instance
//	@Description	Asynchronously creates a Valkey instance on the specified shard. Auto-generates a port and password. Optionally creates nested users in the same request. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			tenantID	path		string							true	"Tenant ID"
//	@Param			body		body		request.CreateValkeyInstance	true	"Valkey instance details"
//	@Success		202			{object}	model.ValkeyInstance
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/tenants/{tenantID}/valkey-instances [post]
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

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
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
		Name:        platform.NewName("kv_"),
		ShardID:     &shardID,
		MaxMemoryMB: maxMemoryMB,
		Password:    generatePassword(),
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), instance); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	// Nested user creation
	for _, ur := range req.Users {
		keyPattern := ur.KeyPattern
		if keyPattern == "" {
			keyPattern = "~*"
		}
		now2 := time.Now()
		user := &model.ValkeyUser{
			ID:               platform.NewID(),
			ValkeyInstanceID: instance.ID,
			Username:         ur.Username,
			Password:         ur.Password,
			Privileges:       ur.Privileges,
			KeyPattern:       keyPattern,
			Status:           model.StatusPending,
			CreatedAt:        now2,
			UpdatedAt:        now2,
		}
		if err := h.userSvc.Create(r.Context(), user); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create valkey user %s: %s", ur.Username, err.Error()))
			return
		}
	}

	instance.Password = ""
	response.WriteJSON(w, http.StatusAccepted, instance)
}

// Get godoc
//
//	@Summary		Get a Valkey instance
//	@Description	Returns instance details including port and shard name. The password is redacted.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Valkey instance ID"
//	@Success		200	{object}	model.ValkeyInstance
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/valkey-instances/{id} [get]
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

	if instance.TenantID != nil {
		if !checkTenantBrand(w, r, h.tenantSvc, *instance.TenantID) {
			return
		}
	}

	instance.Password = ""
	response.WriteJSON(w, http.StatusOK, instance)
}

// Delete godoc
//
//	@Summary		Delete a Valkey instance
//	@Description	Asynchronously deletes a Valkey instance. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Valkey instance ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/valkey-instances/{id} [delete]
func (h *ValkeyInstance) Delete(w http.ResponseWriter, r *http.Request) {
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
	if instance.TenantID != nil {
		if !checkTenantBrand(w, r, h.tenantSvc, *instance.TenantID) {
			return
		}
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Migrate godoc
//
//	@Summary		Migrate a Valkey instance to a different shard
//	@Description	Asynchronously moves a Valkey instance to a different shard via a data migration workflow. Returns 202 immediately.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			id		path	string							true	"Valkey instance ID"
//	@Param			body	body	request.MigrateValkeyInstance	true	"Target shard ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/valkey-instances/{id}/migrate [post]
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

	instance, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if instance.TenantID != nil {
		if !checkTenantBrand(w, r, h.tenantSvc, *instance.TenantID) {
			return
		}
	}

	if err := h.svc.Migrate(r.Context(), id, req.TargetShardID); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ReassignTenant godoc
//
//	@Summary		Reassign Valkey instance to a different tenant
//	@Description	Synchronously changes the tenant ownership of a Valkey instance. Pass null tenant_id to detach from any tenant.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			id		path		string									true	"Valkey instance ID"
//	@Param			body	body		request.ReassignValkeyInstanceTenant	true	"New tenant ID"
//	@Success		200		{object}	model.ValkeyInstance
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/valkey-instances/{id}/tenant [put]
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

	instance, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if instance.TenantID != nil {
		if !checkTenantBrand(w, r, h.tenantSvc, *instance.TenantID) {
			return
		}
	}

	if err := h.svc.ReassignTenant(r.Context(), id, req.TenantID); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	instance, err = h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	instance.Password = ""
	response.WriteJSON(w, http.StatusOK, instance)
}

// Retry godoc
//
//	@Summary		Retry a failed Valkey instance
//	@Description	Re-triggers the provisioning workflow for a Valkey instance that previously failed. Returns 202 immediately.
//	@Tags			Valkey Instances
//	@Security		ApiKeyAuth
//	@Param			id path string true "Valkey instance ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/valkey-instances/{id}/retry [post]
func (h *ValkeyInstance) Retry(w http.ResponseWriter, r *http.Request) {
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
	if instance.TenantID != nil {
		if !checkTenantBrand(w, r, h.tenantSvc, *instance.TenantID) {
			return
		}
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

