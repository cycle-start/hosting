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

type Certificate struct {
	svc *core.CertificateService
}

func NewCertificate(svc *core.CertificateService) *Certificate {
	return &Certificate{svc: svc}
}

func (h *Certificate) ListByFQDN(w http.ResponseWriter, r *http.Request) {
	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	certs, err := h.svc.ListByFQDN(r.Context(), fqdnID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range certs {
		certs[i].KeyPEM = ""
	}
	response.WriteJSON(w, http.StatusOK, certs)
}

func (h *Certificate) Upload(w http.ResponseWriter, r *http.Request) {
	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UploadCertificate
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	cert := &model.Certificate{
		ID:        platform.NewID(),
		FQDNID:    fqdnID,
		Type:      model.CertTypeCustom,
		CertPEM:   req.CertPEM,
		KeyPEM:    req.KeyPEM,
		ChainPEM:  req.ChainPEM,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Upload(r.Context(), cert); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cert.KeyPEM = ""
	response.WriteJSON(w, http.StatusAccepted, cert)
}
