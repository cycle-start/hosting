package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func TestNewCertificateService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Upload ----------

func TestCertificateService_Upload_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	now := time.Now()
	expires := now.Add(365 * 24 * time.Hour)
	cert := &model.Certificate{
		ID:        "test-cert-1",
		FQDNID:    "test-fqdn-1",
		CertPEM:   "-----BEGIN CERTIFICATE-----",
		KeyPEM:    "-----BEGIN PRIVATE KEY-----",
		ChainPEM:  "-----BEGIN CERTIFICATE-----",
		IssuedAt:  &now,
		ExpiresAt: &expires,
		Status:    model.StatusPending,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Upload(ctx, cert)
	require.NoError(t, err)
	assert.Equal(t, model.CertTypeCustom, cert.Type, "Upload should set type to custom")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestCertificateService_Upload_SetsTypeToCustom(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	cert := &model.Certificate{
		ID:     "test-cert-1",
		FQDNID: "test-fqdn-1",
		Type:   model.CertTypeLetsEncrypt, // should be overwritten
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Upload(ctx, cert)
	require.NoError(t, err)
	assert.Equal(t, model.CertTypeCustom, cert.Type)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestCertificateService_Upload_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	cert := &model.Certificate{ID: "test-cert-1", FQDNID: "test-fqdn-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Upload(ctx, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert certificate")
	db.AssertExpectations(t)
}

func TestCertificateService_Upload_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	cert := &model.Certificate{ID: "test-cert-1", FQDNID: "test-fqdn-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Upload(ctx, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UploadCustomCertWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestCertificateService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	certID := "test-cert-1"
	fqdnID := "test-fqdn-1"
	now := time.Now().Truncate(time.Microsecond)
	expires := now.Add(365 * 24 * time.Hour)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = certID
		*(dest[1].(*string)) = fqdnID
		*(dest[2].(*string)) = model.CertTypeCustom
		*(dest[3].(*string)) = "cert-pem"
		*(dest[4].(*string)) = "key-pem"
		*(dest[5].(*string)) = "chain-pem"
		*(dest[6].(**time.Time)) = &now
		*(dest[7].(**time.Time)) = &expires
		*(dest[8].(*string)) = model.StatusActive
		*(dest[9].(**string)) = nil // status_message
		*(dest[10].(*bool)) = true
		*(dest[11].(*time.Time)) = now
		*(dest[12].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, certID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, certID, result.ID)
	assert.Equal(t, fqdnID, result.FQDNID)
	assert.Equal(t, model.CertTypeCustom, result.Type)
	assert.Equal(t, "cert-pem", result.CertPEM)
	assert.True(t, result.IsActive)
	db.AssertExpectations(t)
}

func TestCertificateService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-cert")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get certificate")
	db.AssertExpectations(t)
}

// ---------- ListByFQDN ----------

func TestCertificateService_ListByFQDN_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	fqdnID := "test-fqdn-1"
	id1 := "test-cert-1"
	now := time.Now().Truncate(time.Microsecond)
	expires := now.Add(365 * 24 * time.Hour)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = fqdnID
			*(dest[2].(*string)) = model.CertTypeCustom
			*(dest[3].(*string)) = "cert-pem"
			*(dest[4].(*string)) = "key-pem"
			*(dest[5].(*string)) = "chain-pem"
			*(dest[6].(**time.Time)) = &now
			*(dest[7].(**time.Time)) = &expires
			*(dest[8].(*string)) = model.StatusActive
			*(dest[9].(**string)) = nil // status_message
			*(dest[10].(*bool)) = true
			*(dest[11].(*time.Time)) = now
			*(dest[12].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByFQDN(ctx, fqdnID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, id1, result[0].ID)
	assert.True(t, result[0].IsActive)
	db.AssertExpectations(t)
}

func TestCertificateService_ListByFQDN_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByFQDN(ctx, "test-fqdn-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestCertificateService_ListByFQDN_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewCertificateService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))

	result, _, err := svc.ListByFQDN(ctx, "test-fqdn-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list certificates")
	db.AssertExpectations(t)
}
