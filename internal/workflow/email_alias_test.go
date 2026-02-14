package workflow

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// ---------- CreateEmailAliasWorkflow ----------

type CreateEmailAliasWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateEmailAliasWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateEmailAliasWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateEmailAliasWorkflowTestSuite) TestSuccess() {
	aliasID := "test-alias-1"
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"

	alias := model.EmailAlias{
		ID:             aliasID,
		EmailAccountID: accountID,
		Address:        "alias@example.com",
		Status:         model.StatusPending,
	}

	account := model.EmailAccount{
		ID:      accountID,
		FQDNID:  fqdnID,
		Address: "user@example.com",
	}

	fqdn := model.FQDN{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, BrandID: "test-brand", ClusterID: clusterID}
	clusterConfig, _ := json.Marshal(map[string]string{
		"stalwart_url":   "https://mail.example.com",
		"stalwart_token": "admin-token",
	})
	cluster := model.Cluster{ID: clusterID, Config: clusterConfig}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAliasByID", mock.Anything, aliasID).Return(&alias, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartAddAlias", mock.Anything, activity.StalwartAliasParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		AccountName: "user@example.com",
		Address:     "alias@example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAliasWorkflow, aliasID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateEmailAliasWorkflowTestSuite) TestGetAliasFails_SetsStatusFailed() {
	aliasID := "test-alias-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAliasByID", mock.Anything, aliasID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_aliases", aliasID)).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAliasWorkflow, aliasID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateEmailAliasWorkflowTestSuite) TestStalwartAddAliasFails_SetsStatusFailed() {
	aliasID := "test-alias-3"
	accountID := "test-account-3"
	fqdnID := "test-fqdn-3"
	webrootID := "test-webroot-3"
	tenantID := "test-tenant-3"
	clusterID := "test-cluster-3"

	alias := model.EmailAlias{ID: aliasID, EmailAccountID: accountID, Address: "alias@example.com"}
	account := model.EmailAccount{ID: accountID, FQDNID: fqdnID, Address: "user@example.com"}
	fqdn := model.FQDN{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, BrandID: "test-brand", ClusterID: clusterID}
	clusterConfig, _ := json.Marshal(map[string]string{
		"stalwart_url":   "https://mail.example.com",
		"stalwart_token": "admin-token",
	})
	cluster := model.Cluster{ID: clusterID, Config: clusterConfig}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAliasByID", mock.Anything, aliasID).Return(&alias, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartAddAlias", mock.Anything, activity.StalwartAliasParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		AccountName: "user@example.com",
		Address:     "alias@example.com",
	}).Return(fmt.Errorf("stalwart error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_aliases", aliasID)).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAliasWorkflow, aliasID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteEmailAliasWorkflow ----------

type DeleteEmailAliasWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteEmailAliasWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteEmailAliasWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteEmailAliasWorkflowTestSuite) TestSuccess() {
	aliasID := "test-alias-1"
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"

	alias := model.EmailAlias{ID: aliasID, EmailAccountID: accountID, Address: "alias@example.com"}
	account := model.EmailAccount{ID: accountID, FQDNID: fqdnID, Address: "user@example.com"}
	fqdn := model.FQDN{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, BrandID: "test-brand", ClusterID: clusterID}
	clusterConfig, _ := json.Marshal(map[string]string{
		"stalwart_url":   "https://mail.example.com",
		"stalwart_token": "admin-token",
	})
	cluster := model.Cluster{ID: clusterID, Config: clusterConfig}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAliasByID", mock.Anything, aliasID).Return(&alias, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartRemoveAlias", mock.Anything, activity.StalwartAliasParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		AccountName: "user@example.com",
		Address:     "alias@example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusDeleted,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAliasWorkflow, aliasID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteEmailAliasWorkflowTestSuite) TestGetAliasFails_SetsStatusFailed() {
	aliasID := "test-alias-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_aliases", ID: aliasID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAliasByID", mock.Anything, aliasID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_aliases", aliasID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAliasWorkflow, aliasID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateEmailAliasWorkflow(t *testing.T) {
	suite.Run(t, new(CreateEmailAliasWorkflowTestSuite))
}

func TestDeleteEmailAliasWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteEmailAliasWorkflowTestSuite))
}
