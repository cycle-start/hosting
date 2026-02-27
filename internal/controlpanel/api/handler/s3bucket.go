package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type S3BucketHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewS3BucketHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *S3BucketHandler {
	return &S3BucketHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

// ListByCustomer fetches S3 buckets across all tenants the customer has subscriptions with the "s3" module.
//
//	@Summary      List S3 buckets by customer
//	@Description  Fetches S3 buckets across all tenants the customer has subscriptions with the "s3" module
//	@Tags         S3 Buckets
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.S3Bucket
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/s3-buckets [get]
func (h *S3BucketHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.S3Bucket](w, r, h.customerSvc, h.subscriptionSvc, "s3", func(tid string) ([]hosting.S3Bucket, error) {
		return h.hostingClient.ListS3BucketsByTenant(r.Context(), tid)
	})
}

// Get returns a single S3 bucket by ID, with authorization check.
//
//	@Summary      Get an S3 bucket
//	@Description  Returns a single S3 bucket by ID, with authorization check
//	@Tags         S3 Buckets
//	@Produce      json
//	@Param        id   path      string  true  "S3 Bucket ID"
//	@Success      200  {object}  hosting.S3Bucket
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /s3-buckets/{id} [get]
func (h *S3BucketHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.hostingClient.GetS3Bucket(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "s3 bucket not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, bucket.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, bucket)
}

// Update updates an S3 bucket.
//
//	@Summary      Update an S3 bucket
//	@Description  Updates an S3 bucket
//	@Tags         S3 Buckets
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "S3 Bucket ID"
//	@Param        body  body      object  true  "Bucket update payload"
//	@Success      200   {object}  hosting.S3Bucket
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /s3-buckets/{id} [put]
func (h *S3BucketHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.hostingClient.GetS3Bucket(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "s3 bucket not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, bucket.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.hostingClient.UpdateS3Bucket(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update s3 bucket")
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

// Delete removes an S3 bucket by ID.
//
//	@Summary      Delete an S3 bucket
//	@Description  Removes an S3 bucket by ID
//	@Tags         S3 Buckets
//	@Produce      json
//	@Param        id   path      string  true  "S3 Bucket ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /s3-buckets/{id} [delete]
func (h *S3BucketHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.hostingClient.GetS3Bucket(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "s3 bucket not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, bucket.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteS3Bucket(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete s3 bucket")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAccessKeys returns all access keys for an S3 bucket.
//
//	@Summary      List S3 access keys
//	@Description  Returns all access keys for an S3 bucket
//	@Tags         S3 Buckets
//	@Produce      json
//	@Param        id   path      string  true  "S3 Bucket ID"
//	@Success      200  {object}  map[string][]hosting.S3AccessKey
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /s3-buckets/{id}/access-keys [get]
func (h *S3BucketHandler) ListAccessKeys(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.hostingClient.GetS3Bucket(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "s3 bucket not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, bucket.TenantID) {
		return
	}

	keys, err := h.hostingClient.ListS3AccessKeys(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch access keys")
		return
	}
	if keys == nil {
		keys = []hosting.S3AccessKey{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": keys})
}

// CreateAccessKey creates an access key for an S3 bucket.
//
//	@Summary      Create an S3 access key
//	@Description  Creates an access key for an S3 bucket
//	@Tags         S3 Buckets
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "S3 Bucket ID"
//	@Param        body  body      object  true  "Access key creation payload"
//	@Success      201   {object}  hosting.S3AccessKey
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /s3-buckets/{id}/access-keys [post]
func (h *S3BucketHandler) CreateAccessKey(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.hostingClient.GetS3Bucket(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "s3 bucket not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, bucket.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key, err := h.hostingClient.CreateS3AccessKey(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create access key")
		return
	}

	response.WriteJSON(w, http.StatusCreated, key)
}

// DeleteAccessKey deletes an access key for an S3 bucket.
//
//	@Summary      Delete an S3 access key
//	@Description  Deletes an access key for an S3 bucket
//	@Tags         S3 Buckets
//	@Produce      json
//	@Param        id     path      string  true  "S3 Bucket ID"
//	@Param        keyId  path      string  true  "Access Key ID"
//	@Success      204    "No Content"
//	@Failure      400    {object}  response.ErrorResponse
//	@Failure      403    {object}  response.ErrorResponse
//	@Failure      404    {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /s3-buckets/{id}/access-keys/{keyId} [delete]
func (h *S3BucketHandler) DeleteAccessKey(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucket, err := h.hostingClient.GetS3Bucket(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "s3 bucket not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, bucket.TenantID) {
		return
	}

	keyID := chi.URLParam(r, "keyId")
	if err := h.hostingClient.DeleteS3AccessKey(r.Context(), keyID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete access key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
