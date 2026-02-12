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

type ValkeyUser struct {
	svc *core.ValkeyUserService
}

func NewValkeyUser(svc *core.ValkeyUserService) *ValkeyUser {
	return &ValkeyUser{svc: svc}
}

func (h *ValkeyUser) ListByInstance(w http.ResponseWriter, r *http.Request) {
	instanceID, err := request.RequireID(chi.URLParam(r, "instanceID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	users, hasMore, err := h.svc.ListByInstance(r.Context(), instanceID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range users {
		users[i].Password = ""
	}
	var nextCursor string
	if hasMore && len(users) > 0 {
		nextCursor = users[len(users)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, users, nextCursor, hasMore)
}

func (h *ValkeyUser) Create(w http.ResponseWriter, r *http.Request) {
	instanceID, err := request.RequireID(chi.URLParam(r, "instanceID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateValkeyUser
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	keyPattern := req.KeyPattern
	if keyPattern == "" {
		keyPattern = "~*"
	}

	now := time.Now()
	user := &model.ValkeyUser{
		ID:               platform.NewID(),
		ValkeyInstanceID: instanceID,
		Username:         req.Username,
		Password:         req.Password,
		Privileges:       req.Privileges,
		KeyPattern:       keyPattern,
		Status:           model.StatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.svc.Create(r.Context(), user); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusAccepted, user)
}

func (h *ValkeyUser) Get(w http.ResponseWriter, r *http.Request) {
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

func (h *ValkeyUser) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateValkeyUser
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
	if req.KeyPattern != "" {
		user.KeyPattern = req.KeyPattern
	}

	if err := h.svc.Update(r.Context(), user); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusAccepted, user)
}

func (h *ValkeyUser) Delete(w http.ResponseWriter, r *http.Request) {
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
