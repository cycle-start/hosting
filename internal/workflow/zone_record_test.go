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

// ---------- CreateZoneRecordWorkflow ----------

type CreateZoneRecordWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateZoneRecordWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateZoneRecordWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestSuccess() {
	priority := 10
	params := model.ZoneRecordParams{
		RecordID:  "test-record-1",
		Name:      "www.example.com",
		Type:      "A",
		Content:   "10.0.0.1",
		TTL:       300,
		Priority:  &priority,
		ManagedBy: model.ManagedByCustom,
		ZoneName:  "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		Priority: &priority,
	}).Return(nil)
	s.env.OnActivity("DeactivateAutoRecords", mock.Anything, activity.DeactivateAutoRecordsParams{
		Name: "www.example.com",
		Type: "A",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestNilPriority_Success() {
	params := model.ZoneRecordParams{
		RecordID:  "test-record-2",
		Name:      "www.example.com",
		Type:      "CNAME",
		Content:   "example.com",
		TTL:       3600,
		Priority:  nil,
		ManagedBy: model.ManagedByCustom,
		ZoneName:  "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "CNAME",
		Content:  "example.com",
		TTL:      3600,
		Priority: nil,
	}).Return(nil)
	s.env.OnActivity("DeactivateAutoRecords", mock.Anything, activity.DeactivateAutoRecordsParams{
		Name: "www.example.com",
		Type: "CNAME",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestAutoManaged_NoDeactivation() {
	params := model.ZoneRecordParams{
		RecordID:  "test-record-auto",
		Name:      "www.example.com",
		Type:      "A",
		Content:   "10.0.0.1",
		TTL:       300,
		ManagedBy: model.ManagedByAuto,
		ZoneName:  "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, mock.Anything).Return(nil)
	// No DeactivateAutoRecords expectation — auto records don't trigger deactivation.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestZoneNotFoundInDNS_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-nf",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(0, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestGetDNSZoneFails_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-3",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(0, fmt.Errorf("dns db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestWriteDNSRecordFails_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-4",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UpdateZoneRecordWorkflow ----------

type UpdateZoneRecordWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateZoneRecordWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateZoneRecordWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateZoneRecordWorkflowTestSuite) TestSuccess() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-1",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.2",
		TTL:      600,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("UpdateDNSRecord", mock.Anything, activity.UpdateDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.2",
		TTL:      600,
		Priority: nil,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateZoneRecordWorkflowTestSuite) TestZoneNotFoundInDNS_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-nf",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.2",
		TTL:      600,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(0, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UpdateZoneRecordWorkflowTestSuite) TestUpdateDNSRecordFails_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-2",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.2",
		TTL:      600,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("UpdateDNSRecord", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UpdateZoneRecordWorkflowTestSuite) TestGetDNSZoneFails_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-3",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.2",
		TTL:      600,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(0, fmt.Errorf("dns db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteZoneRecordWorkflow ----------

type DeleteZoneRecordWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteZoneRecordWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteZoneRecordWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestSuccess() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-1",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecord", mock.Anything, activity.DeleteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "A",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestZoneNotFoundInDNS_SkipsDeleteAndSucceeds() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-nf",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(0, nil)
	// No DeleteDNSRecord expectation — it should be skipped.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestDeleteDNSRecordFails_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-2",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecord", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestGetDNSZoneFails_SetsStatusFailed() {
	params := model.ZoneRecordParams{
		RecordID: "test-record-3",
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		ZoneName: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(0, fmt.Errorf("dns db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("zone_records", params.RecordID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestCustomRecord_ReactivatesAutoRecords() {
	params := model.ZoneRecordParams{
		RecordID:  "test-record-custom",
		Name:      "www.example.com",
		Type:      "A",
		Content:   "10.0.0.1",
		TTL:       300,
		ManagedBy: model.ManagedByCustom,
		ZoneName:  "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecord", mock.Anything, activity.DeleteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "A",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: params.RecordID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("ReactivateAutoRecords", mock.Anything, activity.DeactivateAutoRecordsParams{
		Name: "www.example.com",
		Type: "A",
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, params)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateZoneRecordWorkflow(t *testing.T) {
	suite.Run(t, new(CreateZoneRecordWorkflowTestSuite))
}

func TestUpdateZoneRecordWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateZoneRecordWorkflowTestSuite))
}

func TestDeleteZoneRecordWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteZoneRecordWorkflowTestSuite))
}
