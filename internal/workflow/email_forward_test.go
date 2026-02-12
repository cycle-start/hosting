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

// ---------- CreateEmailForwardWorkflow ----------

type CreateEmailForwardWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateEmailForwardWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateEmailForwardWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateEmailForwardWorkflowTestSuite) TestSuccess() {
	forwardID := "test-forward-1"
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"

	fwd := model.EmailForward{
		ID:             forwardID,
		EmailAccountID: accountID,
		Destination:    "bob@gmail.com",
		KeepCopy:       true,
		Status:         model.StatusPending,
	}

	account := model.EmailAccount{
		ID:      accountID,
		FQDNID:  fqdnID,
		Address: "user@example.com",
	}

	fqdn := model.FQDN{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, ClusterID: clusterID}
	clusterConfig, _ := json.Marshal(map[string]string{
		"stalwart_url":   "https://mail.example.com",
		"stalwart_token": "admin-token",
	})
	cluster := model.Cluster{ID: clusterID, Config: clusterConfig}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(&fwd, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartSyncForwardScript", mock.Anything, activity.StalwartSyncForwardParams{
		BaseURL:        "https://mail.example.com",
		AdminToken:     "admin-token",
		AccountName:    "user@example.com",
		EmailAccountID: accountID,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailForwardWorkflow, forwardID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateEmailForwardWorkflowTestSuite) TestGetForwardFails_SetsStatusFailed() {
	forwardID := "test-forward-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailForwardWorkflow, forwardID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateEmailForwardWorkflowTestSuite) TestSyncFails_SetsStatusFailed() {
	forwardID := "test-forward-3"
	accountID := "test-account-3"
	fqdnID := "test-fqdn-3"
	webrootID := "test-webroot-3"
	tenantID := "test-tenant-3"
	clusterID := "test-cluster-3"

	fwd := model.EmailForward{ID: forwardID, EmailAccountID: accountID, Destination: "bob@gmail.com", KeepCopy: true}
	account := model.EmailAccount{ID: accountID, FQDNID: fqdnID, Address: "user@example.com"}
	fqdn := model.FQDN{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, ClusterID: clusterID}
	clusterConfig, _ := json.Marshal(map[string]string{
		"stalwart_url":   "https://mail.example.com",
		"stalwart_token": "admin-token",
	})
	cluster := model.Cluster{ID: clusterID, Config: clusterConfig}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(&fwd, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartSyncForwardScript", mock.Anything, activity.StalwartSyncForwardParams{
		BaseURL:        "https://mail.example.com",
		AdminToken:     "admin-token",
		AccountName:    "user@example.com",
		EmailAccountID: accountID,
	}).Return(fmt.Errorf("jmap error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailForwardWorkflow, forwardID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteEmailForwardWorkflow ----------

type DeleteEmailForwardWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteEmailForwardWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteEmailForwardWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteEmailForwardWorkflowTestSuite) TestSuccess() {
	forwardID := "test-forward-1"
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"

	fwd := model.EmailForward{ID: forwardID, EmailAccountID: accountID, Destination: "bob@gmail.com"}
	account := model.EmailAccount{ID: accountID, FQDNID: fqdnID, Address: "user@example.com"}
	fqdn := model.FQDN{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, ClusterID: clusterID}
	clusterConfig, _ := json.Marshal(map[string]string{
		"stalwart_url":   "https://mail.example.com",
		"stalwart_token": "admin-token",
	})
	cluster := model.Cluster{ID: clusterID, Config: clusterConfig}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(&fwd, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("StalwartSyncForwardScript", mock.Anything, activity.StalwartSyncForwardParams{
		BaseURL:        "https://mail.example.com",
		AdminToken:     "admin-token",
		AccountName:    "user@example.com",
		EmailAccountID: accountID,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailForwardWorkflow, forwardID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteEmailForwardWorkflowTestSuite) TestGetForwardFails_SetsStatusFailed() {
	forwardID := "test-forward-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailForwardWorkflow, forwardID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateEmailForwardWorkflow(t *testing.T) {
	suite.Run(t, new(CreateEmailForwardWorkflowTestSuite))
}

func TestDeleteEmailForwardWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteEmailForwardWorkflowTestSuite))
}
