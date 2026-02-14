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

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(&fwd, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
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
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_forwards", forwardID)).Return(nil)

	s.env.ExecuteWorkflow(CreateEmailForwardWorkflow, forwardID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateEmailForwardWorkflowTestSuite) TestSyncFails_SetsStatusFailed() {
	forwardID := "test-forward-3"
	accountID := "test-account-3"
	fqdnID := "test-fqdn-3"

	fwd := model.EmailForward{ID: forwardID, EmailAccountID: accountID, Destination: "bob@gmail.com", KeepCopy: true}
	account := model.EmailAccount{ID: accountID, FQDNID: fqdnID, Address: "user@example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(&fwd, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
	s.env.OnActivity("StalwartSyncForwardScript", mock.Anything, activity.StalwartSyncForwardParams{
		BaseURL:        "https://mail.example.com",
		AdminToken:     "admin-token",
		AccountName:    "user@example.com",
		EmailAccountID: accountID,
	}).Return(fmt.Errorf("jmap error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_forwards", forwardID)).Return(nil)

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

	fwd := model.EmailForward{ID: forwardID, EmailAccountID: accountID, Destination: "bob@gmail.com"}
	account := model.EmailAccount{ID: accountID, FQDNID: fqdnID, Address: "user@example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "email_forwards", ID: forwardID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetEmailForwardByID", mock.Anything, forwardID).Return(&fwd, nil)
	s.env.OnActivity("GetEmailAccountByID", mock.Anything, accountID).Return(&account, nil)
	s.env.OnActivity("GetStalwartContext", mock.Anything, fqdnID).Return(&activity.StalwartContext{
		StalwartURL:   "https://mail.example.com",
		StalwartToken: "admin-token",
		FQDNID:        fqdnID,
		FQDN:          "example.com",
	}, nil)
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
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("email_forwards", forwardID)).Return(nil)

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
