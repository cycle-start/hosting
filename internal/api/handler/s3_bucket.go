package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type S3Bucket struct {
	svc       *core.S3BucketService
	keySvc    *core.S3AccessKeyService
	tenantSvc *core.TenantService
}

func NewS3Bucket(svc *core.S3BucketService, keySvc *core.S3AccessKeyService, tenantSvc *core.TenantService) *S3Bucket {
	return &S3Bucket{svc: svc, keySvc: keySvc, tenantSvc: tenantSvc}
}

// ListByTenant godoc
//
//	@Summary		List S3 buckets for a tenant
//	@Description	Returns a paginated list of S3 buckets for a tenant. Supports search, status filtering, and sorting.
//	@Tags			S3 Buckets
//	@Security		ApiKeyAuth
//	@Param			tenantID	path		string	true	"Tenant ID"
//	@Param			limit		query		int		false	"Page size"						default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Param			search		query		string	false	"Search term"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			sort		query		string	false	"Sort field"					default(created_at)
//	@Param			order		query		string	false	"Sort order (asc or desc)"		default(asc)
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.S3Bucket}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/tenants/{tenantID}/s3-buckets [get]
func (h *S3Bucket) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	params := request.ParseListParams(r, "created_at")

	buckets, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, params)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(buckets) > 0 {
		nextCursor = buckets[len(buckets)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, buckets, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an S3 bucket
//	@Description	Asynchronously creates an S3 bucket on a storage shard via Ceph RADOS Gateway. Bucket names must be unique per tenant. Optionally set public access and quota. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			S3 Buckets
//	@Security		ApiKeyAuth
//	@Param			tenantID	path		string					true	"Tenant ID"
//	@Param			body		body		request.CreateS3Bucket	true	"S3 bucket details"
//	@Success		202			{object}	model.S3Bucket
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/tenants/{tenantID}/s3-buckets [post]
func (h *S3Bucket) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateS3Bucket
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	now := time.Now()
	shardID := req.ShardID
	bucket := &model.S3Bucket{
		ID:             platform.NewName("s3"),
		TenantID:       tenantID,
		SubscriptionID: req.SubscriptionID,
		ShardID:        &shardID,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.Public != nil && *req.Public {
		bucket.Public = true
	}
	if req.QuotaBytes != nil {
		bucket.QuotaBytes = *req.QuotaBytes
	}

	if err := h.svc.Create(r.Context(), bucket); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, bucket)
}

// Get godoc
//
//	@Summary		Get an S3 bucket
//	@Description	Returns the details of a single S3 bucket by ID.
//	@Tags			S3 Buckets
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"S3 bucket ID"
//	@Success		200	{object}	model.S3Bucket
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/s3-buckets/{id} [get]
func (h *S3Bucket) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, bucket.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, bucket)
}

// Update godoc
//
//	@Summary		Update an S3 bucket
//	@Description	Asynchronously updates a bucket's public access flag and/or quota. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			S3 Buckets
//	@Security		ApiKeyAuth
//	@Param			id		path		string					true	"S3 bucket ID"
//	@Param			body	body		request.UpdateS3Bucket	true	"S3 bucket updates"
//	@Success		202		{object}	model.S3Bucket
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/s3-buckets/{id} [put]
func (h *S3Bucket) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateS3Bucket
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, bucket.TenantID) {
		return
	}

	if err := h.svc.Update(r.Context(), id, req.Public, req.QuotaBytes); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	bucket, err = h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, bucket)
}

// Delete godoc
//
//	@Summary		Delete an S3 bucket
//	@Description	Asynchronously deletes an S3 bucket and all its objects. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			S3 Buckets
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"S3 bucket ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/s3-buckets/{id} [delete]
func (h *S3Bucket) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, bucket.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed S3 bucket
//	@Description	Re-triggers the provisioning workflow for a bucket that previously failed. Returns 202 immediately.
//	@Tags			S3 Buckets
//	@Security		ApiKeyAuth
//	@Param			id path string true "S3 bucket ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/s3-buckets/{id}/retry [post]
func (h *S3Bucket) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	bucket, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, bucket.TenantID) {
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// CreateNested creates S3 buckets as part of tenant creation.
func (h *S3Bucket) CreateNested(w http.ResponseWriter, r *http.Request, tenantID string, buckets []request.CreateS3BucketNested) error {
	for _, br := range buckets {
		now := time.Now()
		shardID := br.ShardID
		bucket := &model.S3Bucket{
			ID:        platform.NewName("s3"),
			TenantID:  tenantID,
			ShardID:   &shardID,
			Status:    model.StatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if br.Public != nil && *br.Public {
			bucket.Public = true
		}
		if br.QuotaBytes != nil {
			bucket.QuotaBytes = *br.QuotaBytes
		}

		if err := h.svc.Create(r.Context(), bucket); err != nil {
			return fmt.Errorf("create s3 bucket: %w", err)
		}
	}
	return nil
}
