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
