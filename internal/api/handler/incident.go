package handler

import (
	"net/http"
	"strconv"

	"github.com/edvin/hosting/internal/api/middleware"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/go-chi/chi/v5"
)

// actorFromRequest derives an actor string from the API key identity.
func actorFromRequest(r *http.Request) string {
	if identity, ok := r.Context().Value(middleware.APIKeyIdentityKey).(*middleware.APIKeyIdentity); ok {
		return "api-key:" + identity.ID
	}
	return "unknown"
}

type Incident struct {
	svc *core.IncidentService
}

func NewIncident(svc *core.IncidentService) *Incident {
	return &Incident{svc: svc}
}

// List godoc
//
//	@Summary		List incidents
//	@Description	Returns a paginated list of incidents with optional filters.
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			status			query		string	false	"Filter by status"
//	@Param			severity		query		string	false	"Filter by severity"
//	@Param			type			query		string	false	"Filter by type"
//	@Param			resource_type	query		string	false	"Filter by resource type"
//	@Param			resource_id		query		string	false	"Filter by resource ID"
//	@Param			source			query		string	false	"Filter by source"
//	@Param			limit			query		int		false	"Page size"			default(50)
//	@Param			cursor			query		string	false	"Pagination cursor"
//	@Success		200				{object}	response.PaginatedResponse{items=[]model.Incident}
//	@Failure		500				{object}	response.ErrorResponse
//	@Router			/incidents [get]
func (h *Incident) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	filters := core.IncidentFilters{
		Status:       q.Get("status"),
		Severity:     q.Get("severity"),
		Type:         q.Get("type"),
		ResourceType: q.Get("resource_type"),
		ResourceID:   q.Get("resource_id"),
		Source:       q.Get("source"),
	}

	incidents, hasMore, err := h.svc.List(r.Context(), filters, limit, q.Get("cursor"))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(incidents) > 0 {
		nextCursor = incidents[len(incidents)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, incidents, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an incident
//	@Description	Creates a new incident, or returns the existing open one if dedupe_key matches.
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			body	body		request.CreateIncident	true	"Incident details"
//	@Success		201		{object}	model.Incident
//	@Success		200		{object}	model.Incident
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/incidents [post]
func (h *Incident) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateIncident
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	inc := &model.Incident{
		DedupeKey:    req.DedupeKey,
		Type:         req.Type,
		Severity:     req.Severity,
		Title:        req.Title,
		Detail:       req.Detail,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		Source:       req.Source,
	}

	created, err := h.svc.Create(r.Context(), inc)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	if created {
		response.WriteJSON(w, http.StatusCreated, inc)
	} else {
		response.WriteJSON(w, http.StatusOK, inc)
	}
}

// Get godoc
//
//	@Summary		Get an incident
//	@Description	Returns a single incident by ID.
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Incident ID"
//	@Success		200	{object}	model.Incident
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/incidents/{id} [get]
func (h *Incident) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inc, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, inc)
}

// Update godoc
//
//	@Summary		Update an incident
//	@Description	Updates mutable fields on an incident (status, severity, assigned_to).
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			id		path	string					true	"Incident ID"
//	@Param			body	body	request.UpdateIncident	true	"Fields to update"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/incidents/{id} [patch]
func (h *Incident) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req request.UpdateIncident
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Update(r.Context(), id, req.Status, req.Severity, req.AssignedTo); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Resolve godoc
//
//	@Summary		Resolve an incident
//	@Description	Marks an incident as resolved with a resolution message.
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			id		path	string						true	"Incident ID"
//	@Param			body	body	request.ResolveIncident		true	"Resolution details"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/incidents/{id}/resolve [post]
func (h *Incident) Resolve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req request.ResolveIncident
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	actor := actorFromRequest(r)
	if err := h.svc.Resolve(r.Context(), id, req.Resolution, actor); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Escalate godoc
//
//	@Summary		Escalate an incident
//	@Description	Marks an incident as escalated with a reason.
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			id		path	string						true	"Incident ID"
//	@Param			body	body	request.EscalateIncident	true	"Escalation details"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/incidents/{id}/escalate [post]
func (h *Incident) Escalate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req request.EscalateIncident
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	actor := actorFromRequest(r)
	if err := h.svc.Escalate(r.Context(), id, req.Reason, actor); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Cancel godoc
//
//	@Summary		Cancel an incident
//	@Description	Marks an incident as cancelled (false positive).
//	@Tags			Incidents
//	@Security		ApiKeyAuth
//	@Param			id		path	string					true	"Incident ID"
//	@Param			body	body	request.CancelIncident	true	"Cancellation details"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/incidents/{id}/cancel [post]
func (h *Incident) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req request.CancelIncident
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	actor := actorFromRequest(r)
	if err := h.svc.Cancel(r.Context(), id, req.Reason, actor); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListEvents godoc
//
//	@Summary		List incident events
//	@Description	Returns a paginated timeline of events for an incident.
//	@Tags			Incident Events
//	@Security		ApiKeyAuth
//	@Param			id		path		string	true	"Incident ID"
//	@Param			limit	query		int		false	"Page size"			default(50)
//	@Param			cursor	query		string	false	"Pagination cursor"
//	@Success		200		{object}	response.PaginatedResponse{items=[]model.IncidentEvent}
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/incidents/{id}/events [get]
func (h *Incident) ListEvents(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "id")
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	events, hasMore, err := h.svc.ListEvents(r.Context(), incidentID, limit, q.Get("cursor"))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(events) > 0 {
		nextCursor = events[len(events)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, events, nextCursor, hasMore)
}

// AddEvent godoc
//
//	@Summary		Add an incident event
//	@Description	Adds a timeline event to an incident.
//	@Tags			Incident Events
//	@Security		ApiKeyAuth
//	@Param			id		path		string						true	"Incident ID"
//	@Param			body	body		request.AddIncidentEvent	true	"Event details"
//	@Success		201		{object}	model.IncidentEvent
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/incidents/{id}/events [post]
func (h *Incident) AddEvent(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "id")

	var req request.AddIncidentEvent
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	evt := &model.IncidentEvent{
		IncidentID: incidentID,
		Actor:      req.Actor,
		Action:     req.Action,
		Detail:     req.Detail,
		Metadata:   req.Metadata,
	}

	if err := h.svc.AddEvent(r.Context(), evt); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, evt)
}
