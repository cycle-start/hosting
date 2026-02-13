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

type S3AccessKey struct {
	svc *core.S3AccessKeyService
}

func NewS3AccessKey(svc *core.S3AccessKeyService) *S3AccessKey {
	return &S3AccessKey{svc: svc}
}

// ListByBucket godoc
//
//	@Summary		List access keys for an S3 bucket
//	@Tags			S3 Access Keys
//	@Security		ApiKeyAuth
//	@Param			bucketID	path		string	true	"S3 bucket ID"
//	@Param			limit		query		int		false	"Page size"	default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.S3AccessKey}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/s3-buckets/{bucketID}/access-keys [get]
func (h *S3AccessKey) ListByBucket(w http.ResponseWriter, r *http.Request) {
	bucketID, err := request.RequireID(chi.URLParam(r, "bucketID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	keys, hasMore, err := h.svc.ListByBucket(r.Context(), bucketID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range keys {
		keys[i].SecretAccessKey = ""
	}
	var nextCursor string
	if hasMore && len(keys) > 0 {
		nextCursor = keys[len(keys)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, keys, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an S3 access key
//	@Tags			S3 Access Keys
//	@Security		ApiKeyAuth
//	@Param			bucketID	path		string						true	"S3 bucket ID"
//	@Param			body		body		request.CreateS3AccessKey	true	"Access key details"
//	@Success		201			{object}	model.S3AccessKey
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/s3-buckets/{bucketID}/access-keys [post]
func (h *S3AccessKey) Create(w http.ResponseWriter, r *http.Request) {
	bucketID, err := request.RequireID(chi.URLParam(r, "bucketID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateS3AccessKey
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	permissions := req.Permissions
	if permissions == "" {
		permissions = "read-write"
	}

	now := time.Now()
	key := &model.S3AccessKey{
		ID:          platform.NewID(),
		S3BucketID:  bucketID,
		Permissions: permissions,
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), key); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return with secret visible (only shown on creation)
	response.WriteJSON(w, http.StatusCreated, key)
}

// Delete godoc
//
//	@Summary		Delete an S3 access key
//	@Tags			S3 Access Keys
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"S3 access key ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/s3-access-keys/{id} [delete]
func (h *S3AccessKey) Delete(w http.ResponseWriter, r *http.Request) {
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
