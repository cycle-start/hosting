package handler

import (
	"encoding/json"
	"net/http"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type PlatformConfig struct {
	svc *core.PlatformConfigService
}

func NewPlatformConfig(svc *core.PlatformConfigService) *PlatformConfig {
	return &PlatformConfig{svc: svc}
}

// Get godoc
//
//	@Summary		Get all platform configuration
//	@Tags			Platform Config
//	@Security		ApiKeyAuth
//	@Success		200	{object}	map[string]string
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/platform/config [get]
func (h *PlatformConfig) Get(w http.ResponseWriter, r *http.Request) {
	configs, err := h.svc.GetAll(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, configs)
}

// Update godoc
//
//	@Summary		Update platform configuration
//	@Tags			Platform Config
//	@Security		ApiKeyAuth
//	@Param			body	body		map[string]string	true	"Config key-value pairs"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/platform/config [put]
func (h *PlatformConfig) Update(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	for key, value := range body {
		if err := h.svc.Set(r.Context(), key, value); err != nil {
			response.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	configs, err := h.svc.GetAll(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, configs)
}
