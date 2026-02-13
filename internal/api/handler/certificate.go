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

// ListByFQDN godoc
//
//	@Summary		List certificates for an FQDN
//	@Tags			Certificates
//	@Security		ApiKeyAuth
//	@Param			fqdnID path string true "FQDN ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.Certificate}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/fqdns/{fqdnID}/certificates [get]
func (h *Certificate) ListByFQDN(w http.ResponseWriter, r *http.Request) {
	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	certs, hasMore, err := h.svc.ListByFQDN(r.Context(), fqdnID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range certs {
		certs[i].KeyPEM = ""
	}
	var nextCursor string
	if hasMore && len(certs) > 0 {
		nextCursor = certs[len(certs)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, certs, nextCursor, hasMore)
}

// Upload godoc
//
//	@Summary		Upload a custom certificate
//	@Tags			Certificates
//	@Security		ApiKeyAuth
//	@Param			fqdnID path string true "FQDN ID"
//	@Param			body body request.UploadCertificate true "Certificate details"
//	@Success		202 {object} model.Certificate
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/fqdns/{fqdnID}/certificates [post]
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
