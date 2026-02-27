package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
)

type Partner struct{}

func NewPartner() *Partner {
	return &Partner{}
}

// Get returns the partner resolved from the request hostname.
func (h *Partner) Get(w http.ResponseWriter, r *http.Request) {
	partner := middleware.GetPartner(r.Context())
	if partner == nil {
		response.WriteError(w, http.StatusNotFound, "unknown partner")
		return
	}
	response.WriteJSON(w, http.StatusOK, partner)
}
