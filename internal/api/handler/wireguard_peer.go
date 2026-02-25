package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/go-chi/chi/v5"
)

type WireGuardPeer struct {
	svc       *core.WireGuardPeerService
	tenantSvc *core.TenantService
}

func NewWireGuardPeer(svc *core.WireGuardPeerService, tenantSvc *core.TenantService) *WireGuardPeer {
	return &WireGuardPeer{svc: svc, tenantSvc: tenantSvc}
}

func (h *WireGuardPeer) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	pg := request.ParsePagination(r)

	peers, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(peers) > 0 {
		nextCursor = peers[len(peers)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, peers, nextCursor, hasMore)
}

func (h *WireGuardPeer) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateWireGuardPeer
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	peer := &model.WireGuardPeer{
		TenantID:       tenantID,
		SubscriptionID: req.SubscriptionID,
		Name:           req.Name,
	}

	result, err := h.svc.Create(r.Context(), peer)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, result)
}

func (h *WireGuardPeer) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	peer, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, peer.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, peer)
}

func (h *WireGuardPeer) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	peer, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, peer.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *WireGuardPeer) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	peer, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, peer.TenantID) {
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
