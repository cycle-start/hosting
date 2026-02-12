package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// ---------- ProvisionLECertWorkflow ----------

type ProvisionLECertWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *ProvisionLECertWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *ProvisionLECertWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *ProvisionLECertWorkflowTestSuite) TestSuccess() {
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  webrootID,
		SSLEnabled: true,
	}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("CreateCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("StoreCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, activity.InstallCertificateParams{
		FQDN:     "secure.example.com",
		CertPEM:  "PLACEHOLDER_CERT_PEM",
		KeyPEM:   "PLACEHOLDER_KEY_PEM",
		ChainPEM: "PLACEHOLDER_CHAIN_PEM",
	}).Return(nil)
	s.env.OnActivity("DeactivateOtherCerts", mock.Anything, fqdnID, mock.Anything).Return(nil)
	s.env.OnActivity("ActivateCertificate", mock.Anything, mock.Anything).Return(nil)

	s.env.ExecuteWorkflow(ProvisionLECertWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProvisionLECertWorkflowTestSuite) TestGetFQDNFails() {
	fqdnID := "test-fqdn-2"

	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(nil, fmt.Errorf("not found"))

	s.env.ExecuteWorkflow(ProvisionLECertWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ProvisionLECertWorkflowTestSuite) TestStoreCertificateFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-3"
	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  "test-webroot-3",
		SSLEnabled: true,
	}

	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("CreateCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("StoreCertificate", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(ProvisionLECertWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ProvisionLECertWorkflowTestSuite) TestInstallCertificateFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-4"
	shardID := "test-shard-4"
	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  "test-webroot-4",
		SSLEnabled: true,
	}
	webroot := model.Webroot{ID: "test-webroot-4", TenantID: "test-tenant-4"}
	tenant := model.Tenant{ID: "test-tenant-4", Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("CreateCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("StoreCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-4").Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, "test-tenant-4").Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))

	s.env.ExecuteWorkflow(ProvisionLECertWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ProvisionLECertWorkflowTestSuite) TestDeactivateOtherCertsFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-5"
	shardID := "test-shard-5"
	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  "test-webroot-5",
		SSLEnabled: true,
	}
	webroot := model.Webroot{ID: "test-webroot-5", TenantID: "test-tenant-5"}
	tenant := model.Tenant{ID: "test-tenant-5", Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("CreateCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("StoreCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-5").Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, "test-tenant-5").Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("DeactivateOtherCerts", mock.Anything, fqdnID, mock.Anything).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(ProvisionLECertWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ProvisionLECertWorkflowTestSuite) TestActivateCertificateFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-6"
	shardID := "test-shard-6"
	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  "test-webroot-6",
		SSLEnabled: true,
	}
	webroot := model.Webroot{ID: "test-webroot-6", TenantID: "test-tenant-6"}
	tenant := model.Tenant{ID: "test-tenant-6", Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("CreateCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("StoreCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-6").Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, "test-tenant-6").Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("DeactivateOtherCerts", mock.Anything, fqdnID, mock.Anything).Return(nil)
	s.env.OnActivity("ActivateCertificate", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(ProvisionLECertWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UploadCustomCertWorkflow ----------

type UploadCustomCertWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UploadCustomCertWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UploadCustomCertWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UploadCustomCertWorkflowTestSuite) TestSuccess() {
	certID := "test-cert-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"

	cert := model.Certificate{
		ID:       certID,
		FQDNID:   fqdnID,
		Type:     model.CertTypeCustom,
		CertPEM:  "CERT_PEM_DATA",
		KeyPEM:   "KEY_PEM_DATA",
		ChainPEM: "CHAIN_PEM_DATA",
	}
	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "custom.example.com",
		WebrootID: webrootID,
	}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetCertificateByID", mock.Anything, certID).Return(&cert, nil)
	s.env.OnActivity("ValidateCustomCert", mock.Anything, "CERT_PEM_DATA", "KEY_PEM_DATA").Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, activity.InstallCertificateParams{
		FQDN:     "custom.example.com",
		CertPEM:  "CERT_PEM_DATA",
		KeyPEM:   "KEY_PEM_DATA",
		ChainPEM: "CHAIN_PEM_DATA",
	}).Return(nil)
	s.env.OnActivity("DeactivateOtherCerts", mock.Anything, fqdnID, certID).Return(nil)
	s.env.OnActivity("ActivateCertificate", mock.Anything, certID).Return(nil)

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UploadCustomCertWorkflowTestSuite) TestGetCertificateFails_SetsStatusFailed() {
	certID := "test-cert-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetCertificateByID", mock.Anything, certID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UploadCustomCertWorkflowTestSuite) TestValidationFails_SetsStatusFailed() {
	certID := "test-cert-3"
	fqdnID := "test-fqdn-3"

	cert := model.Certificate{
		ID:      certID,
		FQDNID:  fqdnID,
		Type:    model.CertTypeCustom,
		CertPEM: "BAD_CERT",
		KeyPEM:  "BAD_KEY",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetCertificateByID", mock.Anything, certID).Return(&cert, nil)
	s.env.OnActivity("ValidateCustomCert", mock.Anything, "BAD_CERT", "BAD_KEY").Return(fmt.Errorf("cert and key do not match"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UploadCustomCertWorkflowTestSuite) TestInstallFails_SetsStatusFailed() {
	certID := "test-cert-4"
	fqdnID := "test-fqdn-4"
	shardID := "test-shard-4"

	cert := model.Certificate{
		ID:       certID,
		FQDNID:   fqdnID,
		Type:     model.CertTypeCustom,
		CertPEM:  "CERT_PEM_DATA",
		KeyPEM:   "KEY_PEM_DATA",
		ChainPEM: "CHAIN_PEM_DATA",
	}
	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "custom.example.com",
		WebrootID: "test-webroot-4",
	}
	webroot := model.Webroot{ID: "test-webroot-4", TenantID: "test-tenant-4"}
	tenant := model.Tenant{ID: "test-tenant-4", Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetCertificateByID", mock.Anything, certID).Return(&cert, nil)
	s.env.OnActivity("ValidateCustomCert", mock.Anything, "CERT_PEM_DATA", "KEY_PEM_DATA").Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-4").Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, "test-tenant-4").Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UploadCustomCertWorkflowTestSuite) TestDeactivateOtherCertsFails_SetsStatusFailed() {
	certID := "test-cert-5"
	fqdnID := "test-fqdn-5"
	shardID := "test-shard-5"

	cert := model.Certificate{
		ID:       certID,
		FQDNID:   fqdnID,
		Type:     model.CertTypeCustom,
		CertPEM:  "CERT_PEM_DATA",
		KeyPEM:   "KEY_PEM_DATA",
		ChainPEM: "CHAIN_PEM_DATA",
	}
	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "custom.example.com",
		WebrootID: "test-webroot-5",
	}
	webroot := model.Webroot{ID: "test-webroot-5", TenantID: "test-tenant-5"}
	tenant := model.Tenant{ID: "test-tenant-5", Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetCertificateByID", mock.Anything, certID).Return(&cert, nil)
	s.env.OnActivity("ValidateCustomCert", mock.Anything, "CERT_PEM_DATA", "KEY_PEM_DATA").Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-5").Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, "test-tenant-5").Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("DeactivateOtherCerts", mock.Anything, fqdnID, certID).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UploadCustomCertWorkflowTestSuite) TestActivateCertFails_SetsStatusFailed() {
	certID := "test-cert-6"
	fqdnID := "test-fqdn-6"
	shardID := "test-shard-6"

	cert := model.Certificate{
		ID:       certID,
		FQDNID:   fqdnID,
		Type:     model.CertTypeCustom,
		CertPEM:  "CERT_PEM_DATA",
		KeyPEM:   "KEY_PEM_DATA",
		ChainPEM: "CHAIN_PEM_DATA",
	}
	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "custom.example.com",
		WebrootID: "test-webroot-6",
	}
	webroot := model.Webroot{ID: "test-webroot-6", TenantID: "test-tenant-6"}
	tenant := model.Tenant{ID: "test-tenant-6", Name: "testuser", ShardID: &shardID}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetCertificateByID", mock.Anything, certID).Return(&cert, nil)
	s.env.OnActivity("ValidateCustomCert", mock.Anything, "CERT_PEM_DATA", "KEY_PEM_DATA").Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-6").Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, "test-tenant-6").Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("InstallCertificate", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("DeactivateOtherCerts", mock.Anything, fqdnID, certID).Return(nil)
	s.env.OnActivity("ActivateCertificate", mock.Anything, certID).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UploadCustomCertWorkflowTestSuite) TestSetProvisioningFails() {
	certID := "test-cert-7"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "certificates", ID: certID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(UploadCustomCertWorkflow, certID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- RenewLECertWorkflow (stub) ----------

type RenewLECertWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RenewLECertWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *RenewLECertWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *RenewLECertWorkflowTestSuite) TestStub_Succeeds() {
	s.env.ExecuteWorkflow(RenewLECertWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// ---------- CleanupExpiredCertsWorkflow (stub) ----------

type CleanupExpiredCertsWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CleanupExpiredCertsWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CleanupExpiredCertsWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CleanupExpiredCertsWorkflowTestSuite) TestStub_Succeeds() {
	s.env.ExecuteWorkflow(CleanupExpiredCertsWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestProvisionLECertWorkflow(t *testing.T) {
	suite.Run(t, new(ProvisionLECertWorkflowTestSuite))
}

func TestUploadCustomCertWorkflow(t *testing.T) {
	suite.Run(t, new(UploadCustomCertWorkflowTestSuite))
}

func TestRenewLECertWorkflow(t *testing.T) {
	suite.Run(t, new(RenewLECertWorkflowTestSuite))
}

func TestCleanupExpiredCertsWorkflow(t *testing.T) {
	suite.Run(t, new(CleanupExpiredCertsWorkflowTestSuite))
}
