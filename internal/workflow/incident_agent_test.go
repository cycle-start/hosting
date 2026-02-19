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

// ---------- ProcessIncidentQueueWorkflow ----------

type ProcessIncidentQueueWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *ProcessIncidentQueueWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *ProcessIncidentQueueWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *ProcessIncidentQueueWorkflowTestSuite) TestNoIncidents() {
	s.env.OnActivity("GetAgentConfig", mock.Anything).
		Return(&activity.AgentConfig{
			SystemPrompt:    "test prompt",
			TypeConcurrency: map[string]int{},
		}, nil)

	s.env.OnActivity("ListUnassignedOpenIncidents", mock.Anything).
		Return([]activity.UnassignedIncident{}, nil)

	s.env.ExecuteWorkflow(ProcessIncidentQueueWorkflow, ProcessIncidentQueueParams{
		MaxConcurrent:      3,
		FollowerConcurrent: 5,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProcessIncidentQueueWorkflowTestSuite) TestSingleIncident() {
	s.env.OnActivity("GetAgentConfig", mock.Anything).
		Return(&activity.AgentConfig{
			SystemPrompt:    "test prompt",
			TypeConcurrency: map[string]int{},
		}, nil)

	s.env.OnActivity("ListUnassignedOpenIncidents", mock.Anything).
		Return([]activity.UnassignedIncident{
			{ID: "inc-1", Type: "disk_pressure", Severity: "warning", Title: "Disk usage high"},
		}, nil)

	// Leader claim succeeds.
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-1").
		Return(true, nil)

	// Leader investigation via child workflow — resolved with hint.
	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-1", "test prompt", "").
		Return(activity.InvestigateIncidentResult{
			Outcome:        "resolved",
			Turns:          2,
			Summary:        "Cleared temp files",
			ResolutionHint: "Delete /tmp/cache/* to free space",
		}, nil)

	s.env.ExecuteWorkflow(ProcessIncidentQueueWorkflow, ProcessIncidentQueueParams{
		MaxConcurrent:      3,
		FollowerConcurrent: 5,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProcessIncidentQueueWorkflowTestSuite) TestGroupedIncidents() {
	s.env.OnActivity("GetAgentConfig", mock.Anything).
		Return(&activity.AgentConfig{
			SystemPrompt:    "test prompt",
			TypeConcurrency: map[string]int{},
		}, nil)

	// Three incidents of the same type — first is leader, other two are followers.
	s.env.OnActivity("ListUnassignedOpenIncidents", mock.Anything).
		Return([]activity.UnassignedIncident{
			{ID: "inc-1", Type: "disk_pressure", Severity: "critical", Title: "Disk critical on web-0"},
			{ID: "inc-2", Type: "disk_pressure", Severity: "warning", Title: "Disk warning on web-1"},
			{ID: "inc-3", Type: "disk_pressure", Severity: "warning", Title: "Disk warning on web-2"},
		}, nil)

	// Leader claim and investigation.
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-1").
		Return(true, nil)

	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-1", "test prompt", "").
		Return(activity.InvestigateIncidentResult{
			Outcome:        "resolved",
			Turns:          3,
			Summary:        "Cleared temp files",
			ResolutionHint: "Delete /tmp/cache/* to free space",
		}, nil)

	// Follower claims.
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-2").
		Return(true, nil)
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-3").
		Return(true, nil)

	// Followers investigated with the leader's hints.
	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-2", "test prompt", "Delete /tmp/cache/* to free space").
		Return(activity.InvestigateIncidentResult{
			Outcome: "resolved",
			Turns:   1,
			Summary: "Applied hint",
		}, nil)
	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-3", "test prompt", "Delete /tmp/cache/* to free space").
		Return(activity.InvestigateIncidentResult{
			Outcome: "resolved",
			Turns:   1,
			Summary: "Applied hint",
		}, nil)

	s.env.ExecuteWorkflow(ProcessIncidentQueueWorkflow, ProcessIncidentQueueParams{
		MaxConcurrent:      3,
		FollowerConcurrent: 5,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProcessIncidentQueueWorkflowTestSuite) TestMultipleTypes() {
	s.env.OnActivity("GetAgentConfig", mock.Anything).
		Return(&activity.AgentConfig{
			SystemPrompt:    "test prompt",
			TypeConcurrency: map[string]int{},
		}, nil)

	// Two different types, one incident each — investigated in parallel as separate leaders.
	s.env.OnActivity("ListUnassignedOpenIncidents", mock.Anything).
		Return([]activity.UnassignedIncident{
			{ID: "inc-1", Type: "disk_pressure", Severity: "warning", Title: "Disk warning"},
			{ID: "inc-2", Type: "node_health_missing", Severity: "critical", Title: "Node unreachable"},
		}, nil)

	// Both claimed.
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-1").
		Return(true, nil)
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-2").
		Return(true, nil)

	// Both investigated as leaders (no hints).
	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-1", "test prompt", "").
		Return(activity.InvestigateIncidentResult{
			Outcome: "resolved",
			Turns:   1,
			Summary: "Fixed disk",
		}, nil)
	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-2", "test prompt", "").
		Return(activity.InvestigateIncidentResult{
			Outcome: "escalated",
			Turns:   2,
			Summary: "Node needs reboot",
		}, nil)

	s.env.ExecuteWorkflow(ProcessIncidentQueueWorkflow, ProcessIncidentQueueParams{
		MaxConcurrent:      3,
		FollowerConcurrent: 5,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProcessIncidentQueueWorkflowTestSuite) TestLeaderEscalated() {
	s.env.OnActivity("GetAgentConfig", mock.Anything).
		Return(&activity.AgentConfig{
			SystemPrompt:    "test prompt",
			TypeConcurrency: map[string]int{},
		}, nil)

	// Two incidents of same type — leader escalates, so no hints for follower.
	s.env.OnActivity("ListUnassignedOpenIncidents", mock.Anything).
		Return([]activity.UnassignedIncident{
			{ID: "inc-1", Type: "convergence_stuck", Severity: "warning", Title: "Shard stuck converging"},
			{ID: "inc-2", Type: "convergence_stuck", Severity: "warning", Title: "Another shard stuck"},
		}, nil)

	// Leader claim and investigation — escalated, no resolution hint.
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-1").
		Return(true, nil)

	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-1", "test prompt", "").
		Return(activity.InvestigateIncidentResult{
			Outcome: "escalated",
			Turns:   4,
			Summary: "Cannot determine root cause",
		}, nil)

	// Follower claimed and investigated without hints (leader did not resolve).
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-2").
		Return(true, nil)

	s.env.OnWorkflow(InvestigateIncidentWorkflow, mock.Anything, "inc-2", "test prompt", "").
		Return(activity.InvestigateIncidentResult{
			Outcome: "resolved",
			Turns:   2,
			Summary: "Fixed it",
		}, nil)

	s.env.ExecuteWorkflow(ProcessIncidentQueueWorkflow, ProcessIncidentQueueParams{
		MaxConcurrent:      3,
		FollowerConcurrent: 5,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProcessIncidentQueueWorkflowTestSuite) TestClaimFailed() {
	s.env.OnActivity("GetAgentConfig", mock.Anything).
		Return(&activity.AgentConfig{
			SystemPrompt:    "test prompt",
			TypeConcurrency: map[string]int{},
		}, nil)

	s.env.OnActivity("ListUnassignedOpenIncidents", mock.Anything).
		Return([]activity.UnassignedIncident{
			{ID: "inc-1", Type: "disk_pressure", Severity: "warning", Title: "Disk warning"},
		}, nil)

	// Claim returns false — incident already claimed by someone else.
	s.env.OnActivity("ClaimIncidentForAgent", mock.Anything, "inc-1").
		Return(false, nil)

	// No child workflow should be started since claim failed.
	// Workflow should complete without error.

	s.env.ExecuteWorkflow(ProcessIncidentQueueWorkflow, ProcessIncidentQueueParams{
		MaxConcurrent:      3,
		FollowerConcurrent: 5,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func TestProcessIncidentQueueWorkflow(t *testing.T) {
	suite.Run(t, new(ProcessIncidentQueueWorkflowTestSuite))
}

// ---------- InvestigateIncidentWorkflow ----------

type InvestigateIncidentWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *InvestigateIncidentWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *InvestigateIncidentWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *InvestigateIncidentWorkflowTestSuite) TestInvestigateSuccess() {
	incCtx := &activity.IncidentContext{
		Incident: model.Incident{
			ID:       "inc-1",
			Type:     "disk_pressure",
			Severity: "warning",
			Status:   model.IncidentInvestigating,
			Title:    "Disk usage high",
		},
		Events: []model.IncidentEvent{},
	}

	s.env.OnActivity("AssembleIncidentContext", mock.Anything, "inc-1").
		Return(incCtx, nil)

	s.env.OnActivity("InvestigateIncident", mock.Anything, mock.MatchedBy(func(p activity.InvestigateIncidentParams) bool {
		return p.SystemPrompt == "test prompt" &&
			p.IncidentContext != nil &&
			p.IncidentContext.Incident.ID == "inc-1" &&
			p.Hints == "try this"
	})).Return(&activity.InvestigateIncidentResult{
		Outcome:        "resolved",
		Turns:          2,
		Summary:        "Cleared temp files",
		ResolutionHint: "Delete /tmp/cache/*",
	}, nil)

	s.env.ExecuteWorkflow(InvestigateIncidentWorkflow, "inc-1", "test prompt", "try this")
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result activity.InvestigateIncidentResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal("resolved", result.Outcome)
	s.Equal(2, result.Turns)
	s.Equal("Cleared temp files", result.Summary)
	s.Equal("Delete /tmp/cache/*", result.ResolutionHint)
}

func (s *InvestigateIncidentWorkflowTestSuite) TestInvestigateMaxTurns() {
	incCtx := &activity.IncidentContext{
		Incident: model.Incident{
			ID:       "inc-1",
			Type:     "convergence_stuck",
			Severity: "warning",
			Status:   model.IncidentInvestigating,
			Title:    "Shard stuck",
		},
		Events: []model.IncidentEvent{},
	}

	s.env.OnActivity("AssembleIncidentContext", mock.Anything, "inc-1").
		Return(incCtx, nil)

	// Investigation returns max_turns outcome.
	s.env.OnActivity("InvestigateIncident", mock.Anything, mock.Anything).
		Return(&activity.InvestigateIncidentResult{
			Outcome: "max_turns",
			Turns:   10,
			Summary: "Agent reached maximum investigation turns",
		}, nil)

	// max_turns triggers escalation.
	s.env.OnActivity("EscalateIncident", mock.Anything, mock.MatchedBy(func(p activity.EscalateIncidentParams) bool {
		return p.IncidentID == "inc-1" &&
			p.Actor == "agent:incident-investigator" &&
			p.Reason == "Agent reached maximum investigation turns without resolving or escalating"
	})).Return(nil)

	s.env.ExecuteWorkflow(InvestigateIncidentWorkflow, "inc-1", "test prompt", "")
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result activity.InvestigateIncidentResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal("max_turns", result.Outcome)
	s.Equal(10, result.Turns)
}

func (s *InvestigateIncidentWorkflowTestSuite) TestAssembleContextFails() {
	s.env.OnActivity("AssembleIncidentContext", mock.Anything, "inc-1").
		Return(nil, fmt.Errorf("incident not found"))

	// Should escalate on assembly failure.
	s.env.OnActivity("EscalateIncident", mock.Anything, mock.MatchedBy(func(p activity.EscalateIncidentParams) bool {
		return p.IncidentID == "inc-1" &&
			p.Actor == "agent:incident-investigator"
	})).Return(nil)

	s.env.ExecuteWorkflow(InvestigateIncidentWorkflow, "inc-1", "test prompt", "")
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func TestInvestigateIncidentWorkflow(t *testing.T) {
	suite.Run(t, new(InvestigateIncidentWorkflowTestSuite))
}
