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
	"github.com/edvin/hosting/internal/stalwart"
)

// ---------- UpdateEmailAutoReplyWorkflow ----------

type UpdateEmailAutoReplyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateEmailAutoReplyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateEmailAutoReplyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateEmailAutoReplyWorkflowTestSuite) TestSuccess() {
	autoReplyID := "test-autoreply-1"
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"

	ar := model.EmailAutoReply{
		ID:             autoReplyID,
		EmailAccountID: accountID,
		Subject:        "Out of office",
		Body:           "I'm on vacation.",
		Enabled:        true,
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
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAutoReplyByID", mock.Anything, autoReplyID).Return(&ar, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartSetVacation", mock.Anything, activity.StalwartVacationParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		AccountName: "user@example.com",
		Vacation: &stalwart.VacationParams{
			Subject: "Out of office",
			Body:    "I'm on vacation.",
			Enabled: true,
		},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(UpdateEmailAutoReplyWorkflow, autoReplyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateEmailAutoReplyWorkflowTestSuite) TestGetAutoReplyFails_SetsStatusFailed() {
	autoReplyID := "test-autoreply-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAutoReplyByID", mock.Anything, autoReplyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UpdateEmailAutoReplyWorkflow, autoReplyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UpdateEmailAutoReplyWorkflowTestSuite) TestSetVacationFails_SetsStatusFailed() {
	autoReplyID := "test-autoreply-3"
	accountID := "test-account-3"
	fqdnID := "test-fqdn-3"
	webrootID := "test-webroot-3"
	tenantID := "test-tenant-3"
	clusterID := "test-cluster-3"

	ar := model.EmailAutoReply{
		ID: autoReplyID, EmailAccountID: accountID, Subject: "OOO", Body: "Gone.", Enabled: true,
	}
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
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAutoReplyByID", mock.Anything, autoReplyID).Return(&ar, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartSetVacation", mock.Anything, activity.StalwartVacationParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		AccountName: "user@example.com",
		Vacation: &stalwart.VacationParams{
			Subject: "OOO",
			Body:    "Gone.",
			Enabled: true,
		},
	}).Return(fmt.Errorf("jmap error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(UpdateEmailAutoReplyWorkflow, autoReplyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteEmailAutoReplyWorkflow ----------

type DeleteEmailAutoReplyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteEmailAutoReplyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteEmailAutoReplyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteEmailAutoReplyWorkflowTestSuite) TestSuccess() {
	autoReplyID := "test-autoreply-1"
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"

	ar := model.EmailAutoReply{ID: autoReplyID, EmailAccountID: accountID}
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
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAutoReplyByID", mock.Anything, autoReplyID).Return(&ar, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("StalwartSetVacation", mock.Anything, activity.StalwartVacationParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		AccountName: "user@example.com",
		Vacation:    (*stalwart.VacationParams)(nil),
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusDeleted,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAutoReplyWorkflow, autoReplyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteEmailAutoReplyWorkflowTestSuite) TestGetAutoReplyFails_SetsStatusFailed() {
	autoReplyID := "test-autoreply-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAutoReplyByID", mock.Anything, autoReplyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_autoreplies", ID: autoReplyID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAutoReplyWorkflow, autoReplyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestUpdateEmailAutoReplyWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateEmailAutoReplyWorkflowTestSuite))
}

func TestDeleteEmailAutoReplyWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteEmailAutoReplyWorkflowTestSuite))
}
