package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type TenantEgressRule struct {
	svc       *core.TenantEgressRuleService
	tenantSvc *core.TenantService
}

func NewTenantEgressRule(svc *core.TenantEgressRuleService, tenantSvc *core.TenantService) *TenantEgressRule {
	return &TenantEgressRule{svc: svc, tenantSvc: tenantSvc}
}

// ListByTenant godoc
//
//	@Summary		List egress rules for a tenant
//	@Description	Returns a paginated list of network egress rules for the specified tenant.
//	@Tags			Tenant Egress Rules
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.TenantEgressRule}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/egress-rules [get]
func (h *TenantEgressRule) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	pg := request.ParsePagination(r)

	rules, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(rules) > 0 {
		nextCursor = rules[len(rules)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, rules, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an egress rule
//	@Description	Adds a network egress rule for a tenant. Rules control which destination CIDRs the tenant's processes can reach. Async — returns 202 and triggers a workflow to sync nftables rules on all shard nodes.
//	@Tags			Tenant Egress Rules
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			body body request.CreateTenantEgressRule true "Egress rule details"
//	@Success		202 {object} model.TenantEgressRule
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/egress-rules [post]
func (h *TenantEgressRule) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateTenantEgressRule
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	now := time.Now()
	rule := &model.TenantEgressRule{
		ID:          platform.NewID(),
		TenantID:    tenantID,
		CIDR:        req.CIDR,
		Description: req.Description,
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), rule); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, rule)
}

// Get godoc
//
//	@Summary		Get an egress rule
//	@Description	Returns a single egress rule by ID.
//	@Tags			Tenant Egress Rules
//	@Security		ApiKeyAuth
//	@Param			id path string true "Egress rule ID"
//	@Success		200 {object} model.TenantEgressRule
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/egress-rules/{id} [get]
func (h *TenantEgressRule) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	rule, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, rule.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, rule)
}

// Delete godoc
//
//	@Summary		Delete an egress rule
//	@Description	Removes a network egress rule. Async — returns 202 and triggers a workflow to remove the nftables rule from all shard nodes.
//	@Tags			Tenant Egress Rules
//	@Security		ApiKeyAuth
//	@Param			id path string true "Egress rule ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/egress-rules/{id} [delete]
func (h *TenantEgressRule) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	rule, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, rule.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed egress rule
//	@Description	Re-triggers the sync workflow for an egress rule in failed state.
//	@Tags			Tenant Egress Rules
//	@Security		ApiKeyAuth
//	@Param			id path string true "Egress rule ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/egress-rules/{id}/retry [post]
func (h *TenantEgressRule) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	rule, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, rule.TenantID) {
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
