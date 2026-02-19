package handler

import (
	"net/http"
	"strconv"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/go-chi/chi/v5"
)

type CapabilityGap struct {
	svc *core.CapabilityGapService
}

func NewCapabilityGap(svc *core.CapabilityGapService) *CapabilityGap {
	return &CapabilityGap{svc: svc}
}

// List godoc
//
//	@Summary		List capability gaps
//	@Description	Returns capability gaps sorted by occurrences descending.
//	@Tags			Capability Gaps
//	@Security		ApiKeyAuth
//	@Param			status		query		string	false	"Filter by status"
//	@Param			category	query		string	false	"Filter by category"
//	@Param			limit		query		int		false	"Page size"			default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.CapabilityGap}
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/capability-gaps [get]
func (h *CapabilityGap) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	gaps, hasMore, err := h.svc.List(r.Context(), q.Get("status"), q.Get("category"), limit, q.Get("cursor"))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(gaps) > 0 {
		nextCursor = gaps[len(gaps)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, gaps, nextCursor, hasMore)
}

// Report godoc
//
//	@Summary		Report a capability gap
//	@Description	Reports a new capability gap or increments occurrences if the tool already exists.
//	@Tags			Capability Gaps
//	@Security		ApiKeyAuth
//	@Param			body	body		request.ReportCapabilityGap	true	"Gap details"
//	@Success		201		{object}	model.CapabilityGap
//	@Success		200		{object}	model.CapabilityGap
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/capability-gaps [post]
func (h *CapabilityGap) Report(w http.ResponseWriter, r *http.Request) {
	var req request.ReportCapabilityGap
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	gap, created, err := h.svc.Report(r.Context(), req.ToolName, req.Description, req.Category, req.IncidentID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	if created {
		response.WriteJSON(w, http.StatusCreated, gap)
	} else {
		response.WriteJSON(w, http.StatusOK, gap)
	}
}

// Update godoc
//
//	@Summary		Update a capability gap
//	@Description	Updates the status of a capability gap.
//	@Tags			Capability Gaps
//	@Security		ApiKeyAuth
//	@Param			id		path	string							true	"Gap ID"
//	@Param			body	body	request.UpdateCapabilityGap		true	"Fields to update"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/capability-gaps/{id} [patch]
func (h *CapabilityGap) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req request.UpdateCapabilityGap
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Update(r.Context(), id, req.Status); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
