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

// ---------- TenantProvisionWorkflow ----------

type TenantProvisionWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *TenantProvisionWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *TenantProvisionWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *TenantProvisionWorkflowTestSuite) TestCallbackFiredOnSuccess() {
	task := model.ProvisionTask{
		WorkflowName: "CreateDatabaseWorkflow",
		WorkflowID:   "create-db-1",
		Arg:          "db-123",
		CallbackURL:  "http://example.com/callback",
		ResourceType: "database",
		ResourceID:   "db-123",
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(model.ProvisionSignalName, task)
	}, 0)

	// Mock child workflow success
	s.env.OnWorkflow(CreateDatabaseWorkflow, mock.Anything, "db-123").Return(nil)

	// Expect callback with active status
	s.env.OnActivity("SendCallback", mock.Anything, mock.MatchedBy(func(params activity.SendCallbackParams) bool {
		return params.URL == "http://example.com/callback" &&
			params.Payload.ResourceType == "database" &&
			params.Payload.ResourceID == "db-123" &&
			params.Payload.Status == model.StatusActive &&
			params.Payload.StatusMessage == ""
	})).Return(nil)

	s.env.ExecuteWorkflow(TenantProvisionWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *TenantProvisionWorkflowTestSuite) TestCallbackFiredOnFailure() {
	task := model.ProvisionTask{
		WorkflowName: "CreateDatabaseWorkflow",
		WorkflowID:   "create-db-2",
		Arg:          "db-456",
		CallbackURL:  "http://example.com/callback",
		ResourceType: "database",
		ResourceID:   "db-456",
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(model.ProvisionSignalName, task)
	}, 0)

	// Mock child workflow failure
	s.env.OnWorkflow(CreateDatabaseWorkflow, mock.Anything, "db-456").Return(fmt.Errorf("db creation failed"))

	// Expect callback with failed status and error message
	s.env.OnActivity("SendCallback", mock.Anything, mock.MatchedBy(func(params activity.SendCallbackParams) bool {
		return params.URL == "http://example.com/callback" &&
			params.Payload.ResourceType == "database" &&
			params.Payload.ResourceID == "db-456" &&
			params.Payload.Status == model.StatusFailed &&
			params.Payload.StatusMessage != ""
	})).Return(nil)

	s.env.ExecuteWorkflow(TenantProvisionWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *TenantProvisionWorkflowTestSuite) TestNoCallbackWhenURLEmpty() {
	task := model.ProvisionTask{
		WorkflowName: "CreateDatabaseWorkflow",
		WorkflowID:   "create-db-3",
		Arg:          "db-789",
		ResourceType: "database",
		ResourceID:   "db-789",
		// CallbackURL intentionally empty
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(model.ProvisionSignalName, task)
	}, 0)

	// Mock child workflow success
	s.env.OnWorkflow(CreateDatabaseWorkflow, mock.Anything, "db-789").Return(nil)

	// No SendCallback mock — if it's called, AssertExpectations will catch it

	s.env.ExecuteWorkflow(TenantProvisionWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *TenantProvisionWorkflowTestSuite) TestCallbackFailureDoesNotBlockOrchestrator() {
	task := model.ProvisionTask{
		WorkflowName: "CreateDatabaseWorkflow",
		WorkflowID:   "create-db-4",
		Arg:          "db-abc",
		CallbackURL:  "http://example.com/callback",
		ResourceType: "database",
		ResourceID:   "db-abc",
	}

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(model.ProvisionSignalName, task)
	}, 0)

	// Child workflow succeeds
	s.env.OnWorkflow(CreateDatabaseWorkflow, mock.Anything, "db-abc").Return(nil)

	// Callback activity fails — should NOT block the orchestrator
	s.env.OnActivity("SendCallback", mock.Anything, mock.Anything).Return(fmt.Errorf("callback endpoint down"))

	s.env.ExecuteWorkflow(TenantProvisionWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	// Orchestrator should still complete without error
	s.NoError(s.env.GetWorkflowError())
}

func (s *TenantProvisionWorkflowTestSuite) TestIdleTimeout() {
	// No signals — workflow should complete after idle timeout
	s.env.ExecuteWorkflow(TenantProvisionWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// ---------- Run ----------

func TestTenantProvisionWorkflow(t *testing.T) {
	suite.Run(t, new(TenantProvisionWorkflowTestSuite))
}
