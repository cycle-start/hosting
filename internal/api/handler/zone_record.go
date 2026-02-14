package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type ZoneRecord struct {
	svc *core.ZoneRecordService
}

func NewZoneRecord(svc *core.ZoneRecordService) *ZoneRecord {
	return &ZoneRecord{svc: svc}
}

// ListByZone godoc
//
//	@Summary		List zone records
//	@Tags			Zone Records
//	@Security		ApiKeyAuth
//	@Param			zoneID	path		string	true	"Zone ID"
//	@Param			limit	query		int		false	"Page size"	default(50)
//	@Param			cursor	query		string	false	"Pagination cursor"
//	@Success		200		{object}	response.PaginatedResponse{items=[]model.ZoneRecord}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/zones/{zoneID}/records [get]
func (h *ZoneRecord) ListByZone(w http.ResponseWriter, r *http.Request) {
	zoneID, err := request.RequireID(chi.URLParam(r, "zoneID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	records, hasMore, err := h.svc.ListByZone(r.Context(), zoneID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(records) > 0 {
		nextCursor = records[len(records)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, records, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a zone record
//	@Tags			Zone Records
//	@Security		ApiKeyAuth
//	@Param			zoneID	path		string						true	"Zone ID"
//	@Param			body	body		request.CreateZoneRecord	true	"Zone record details"
//	@Success		202		{object}	model.ZoneRecord
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/zones/{zoneID}/records [post]
func (h *ZoneRecord) Create(w http.ResponseWriter, r *http.Request) {
	zoneID, err := request.RequireID(chi.URLParam(r, "zoneID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateZoneRecord
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	ttl := req.TTL
	if ttl == 0 {
		ttl = 3600
	}

	now := time.Now()
	record := &model.ZoneRecord{
		ID:        platform.NewID(),
		ZoneID:    zoneID,
		Type:      req.Type,
		Name:      req.Name,
		Content:   req.Content,
		TTL:       ttl,
		Priority:  req.Priority,
		ManagedBy: model.ManagedByUser,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), record); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, record)
}

// Get godoc
//
//	@Summary		Get a zone record
//	@Tags			Zone Records
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Zone record ID"
//	@Success		200	{object}	model.ZoneRecord
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/zone-records/{id} [get]
func (h *ZoneRecord) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, record)
}

// Update godoc
//
//	@Summary		Update a zone record
//	@Tags			Zone Records
//	@Security		ApiKeyAuth
//	@Param			id		path		string						true	"Zone record ID"
//	@Param			body	body		request.UpdateZoneRecord	true	"Zone record updates"
//	@Success		202		{object}	model.ZoneRecord
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/zone-records/{id} [put]
func (h *ZoneRecord) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateZoneRecord
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Content != "" {
		record.Content = req.Content
	}
	if req.TTL != nil {
		record.TTL = *req.TTL
	}
	if req.Priority != nil {
		record.Priority = req.Priority
	}

	if err := h.svc.Update(r.Context(), record); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, record)
}

// Delete godoc
//
//	@Summary		Delete a zone record
//	@Tags			Zone Records
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Zone record ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/zone-records/{id} [delete]
func (h *ZoneRecord) Delete(w http.ResponseWriter, r *http.Request) {
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

// Retry godoc
//
//	@Summary		Retry a failed zone record
//	@Tags			Zone Records
//	@Security		ApiKeyAuth
//	@Param			id path string true "Zone record ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/zone-records/{id}/retry [post]
func (h *ZoneRecord) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
