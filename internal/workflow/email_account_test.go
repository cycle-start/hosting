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

// ---------- CreateEmailAccountWorkflow ----------

type CreateEmailAccountWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateEmailAccountWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateEmailAccountWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateEmailAccountWorkflowTestSuite) TestSuccess() {
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"

	account := model.EmailAccount{
		ID:          accountID,
		FQDNID:      fqdnID,
		Address:     "user@example.com",
		DisplayName: "Test User",
		QuotaBytes:  1073741824,
		Status:      model.StatusPending,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		MailHostname:  "mail.cluster1.example.com",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
	s.env.OnActivity("StalwartCreateDomain", mock.Anything, activity.StalwartDomainParams{
		BaseURL:    "https://mail.example.com",
		AdminToken: "admin-token",
		Domain:     "example.com",
	}).Return(nil)
	s.env.OnActivity("StalwartCreateAccount", mock.Anything, activity.StalwartCreateAccountParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		Address:     "user@example.com",
		DisplayName: "Test User",
		QuotaBytes:  1073741824,
	}).Return(nil)
	s.env.OnActivity("AutoCreateEmailDNSRecords", mock.Anything, activity.AutoCreateEmailDNSRecordsParams{
		FQDN:         "example.com",
		MailHostname: "mail.cluster1.example.com",
		SourceFQDNID: fqdnID,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateEmailAccountWorkflowTestSuite) TestGetEmailAccountFails_SetsStatusFailed() {
	accountID := "test-account-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_accounts", accountID)).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateEmailAccountWorkflowTestSuite) TestGetStalwartContextFails_SetsStatusFailed() {
	accountID := "test-account-3"
	fqdnID := "test-fqdn-3"

	account := model.EmailAccount{
		ID:      accountID,
		FQDNID:  fqdnID,
		Address: "user@example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(nil, fmt.Errorf("stalwart context not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_accounts", accountID)).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateEmailAccountWorkflowTestSuite) TestStalwartCreateAccountFails_SetsStatusFailed() {
	accountID := "test-account-4"
	fqdnID := "test-fqdn-4"

	account := model.EmailAccount{
		ID:          accountID,
		FQDNID:      fqdnID,
		Address:     "user@example.com",
		DisplayName: "Test User",
		QuotaBytes:  1073741824,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
	s.env.OnActivity("StalwartCreateDomain", mock.Anything, activity.StalwartDomainParams{
		BaseURL:    "https://mail.example.com",
		AdminToken: "admin-token",
		Domain:     "example.com",
	}).Return(nil)
	s.env.OnActivity("StalwartCreateAccount", mock.Anything, activity.StalwartCreateAccountParams{
		BaseURL:     "https://mail.example.com",
		AdminToken:  "admin-token",
		Address:     "user@example.com",
		DisplayName: "Test User",
		QuotaBytes:  1073741824,
	}).Return(fmt.Errorf("stalwart error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_accounts", accountID)).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteEmailAccountWorkflow ----------

type DeleteEmailAccountWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteEmailAccountWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteEmailAccountWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteEmailAccountWorkflowTestSuite) TestSuccess_LastAccount_CleansUpDomain() {
	accountID := "test-account-1"
	fqdnID := "test-fqdn-1"

	account := model.EmailAccount{
		ID:      accountID,
		FQDNID:  fqdnID,
		Address: "user@example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
	s.env.OnActivity("StalwartDeleteAccount", mock.Anything, activity.StalwartDeleteAccountParams{
		BaseURL:    "https://mail.example.com",
		AdminToken: "admin-token",
		Address:    "user@example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("CountActiveEmailAccountsByFQDN", mock.Anything, fqdnID).Return(0, nil)
	s.env.OnActivity("StalwartDeleteDomain", mock.Anything, activity.StalwartDomainParams{
		BaseURL:    "https://mail.example.com",
		AdminToken: "admin-token",
		Domain:     "example.com",
	}).Return(nil)
	s.env.OnActivity("AutoDeleteEmailDNSRecords", mock.Anything, "example.com").Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteEmailAccountWorkflowTestSuite) TestSuccess_OtherAccountsRemain_NoDomainCleanup() {
	accountID := "test-account-2"
	fqdnID := "test-fqdn-2"

	account := model.EmailAccount{
		ID:      accountID,
		FQDNID:  fqdnID,
		Address: "user@example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
	s.env.OnActivity("StalwartDeleteAccount", mock.Anything, activity.StalwartDeleteAccountParams{
		BaseURL:    "https://mail.example.com",
		AdminToken: "admin-token",
		Address:    "user@example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusDeleted,
	}).Return(nil)
	// 2 remaining accounts -- no domain cleanup.
	s.env.OnActivity("CountActiveEmailAccountsByFQDN", mock.Anything, fqdnID).Return(2, nil)

	s.env.ExecuteWorkflow(DeleteEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteEmailAccountWorkflowTestSuite) TestGetEmailAccountFails_SetsStatusFailed() {
	accountID := "test-account-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_accounts", accountID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteEmailAccountWorkflowTestSuite) TestStalwartDeleteAccountFails_SetsStatusFailed() {
	accountID := "test-account-4"
	fqdnID := "test-fqdn-4"

	account := model.EmailAccount{
		ID:      accountID,
		FQDNID:  fqdnID,
		Address: "user@example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_accounts", ID: accountID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
	s.env.OnActivity("StalwartDeleteAccount", mock.Anything, activity.StalwartDeleteAccountParams{
		BaseURL:    "https://mail.example.com",
		AdminToken: "admin-token",
		Address:    "user@example.com",
	}).Return(fmt.Errorf("stalwart error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_accounts", accountID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteEmailAccountWorkflow, accountID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateEmailAccountWorkflow(t *testing.T) {
	suite.Run(t, new(CreateEmailAccountWorkflowTestSuite))
}

func TestDeleteEmailAccountWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteEmailAccountWorkflowTestSuite))
}
