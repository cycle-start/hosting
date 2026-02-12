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

type DatabaseUser struct {
	svc *core.DatabaseUserService
}

func NewDatabaseUser(svc *core.DatabaseUserService) *DatabaseUser {
	return &DatabaseUser{svc: svc}
}

func (h *DatabaseUser) ListByDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID, err := request.RequireID(chi.URLParam(r, "databaseID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	users, err := h.svc.ListByDatabase(r.Context(), databaseID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range users {
		users[i].Password = ""
	}
	response.WriteJSON(w, http.StatusOK, users)
}

func (h *DatabaseUser) Create(w http.ResponseWriter, r *http.Request) {
	databaseID, err := request.RequireID(chi.URLParam(r, "databaseID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateDatabaseUser
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	user := &model.DatabaseUser{
		ID:         platform.NewID(),
		DatabaseID: databaseID,
		Username:   req.Username,
		Password:   req.Password,
		Privileges: req.Privileges,
		Status:     model.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.svc.Create(r.Context(), user); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusAccepted, user)
}

func (h *DatabaseUser) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusOK, user)
}

func (h *DatabaseUser) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateDatabaseUser
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Password != "" {
		user.Password = req.Password
	}
	if req.Privileges != nil {
		user.Privileges = req.Privileges
	}

	if err := h.svc.Update(r.Context(), user); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusAccepted, user)
}

func (h *DatabaseUser) Delete(w http.ResponseWriter, r *http.Request) {
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
