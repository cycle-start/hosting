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
	recordID := "test-record-1"
	zoneID := "test-zone-1"
	priority := 10

	record := model.ZoneRecord{
		ID:       recordID,
		ZoneID:   zoneID,
		Type:     "A",
		Name:     "www.example.com",
		Content:  "10.0.0.1",
		TTL:      300,
		Priority: &priority,
	}
	zone := model.Zone{
		ID:   zoneID,
		Name: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "A",
		Content:  "10.0.0.1",
		TTL:      300,
		Priority: &priority,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestNilPriority_Success() {
	recordID := "test-record-2"
	zoneID := "test-zone-2"

	record := model.ZoneRecord{
		ID:       recordID,
		ZoneID:   zoneID,
		Type:     "CNAME",
		Name:     "www.example.com",
		Content:  "example.com",
		TTL:      3600,
		Priority: nil,
	}
	zone := model.Zone{
		ID:   zoneID,
		Name: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "CNAME",
		Content:  "example.com",
		TTL:      3600,
		Priority: nil,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestGetRecordFails_SetsStatusFailed() {
	recordID := "test-record-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateZoneRecordWorkflowTestSuite) TestWriteDNSRecordFails_SetsStatusFailed() {
	recordID := "test-record-4"
	zoneID := "test-zone-4"

	record := model.ZoneRecord{
		ID:      recordID,
		ZoneID:  zoneID,
		Type:    "A",
		Name:    "www.example.com",
		Content: "10.0.0.1",
		TTL:     300,
	}
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("WriteDNSRecord", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneRecordWorkflow, recordID)
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
	recordID := "test-record-1"
	zoneID := "test-zone-1"

	record := model.ZoneRecord{
		ID:       recordID,
		ZoneID:   zoneID,
		Type:     "A",
		Name:     "www.example.com",
		Content:  "10.0.0.2",
		TTL:      600,
		Priority: nil,
	}
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
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
		Table: "zone_records", ID: recordID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateZoneRecordWorkflowTestSuite) TestUpdateDNSRecordFails_SetsStatusFailed() {
	recordID := "test-record-2"
	zoneID := "test-zone-2"

	record := model.ZoneRecord{
		ID:      recordID,
		ZoneID:  zoneID,
		Type:    "A",
		Name:    "www.example.com",
		Content: "10.0.0.2",
		TTL:     600,
	}
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("UpdateDNSRecord", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UpdateZoneRecordWorkflowTestSuite) TestGetRecordFails_SetsStatusFailed() {
	recordID := "test-record-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateZoneRecordWorkflow, recordID)
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
	recordID := "test-record-1"
	zoneID := "test-zone-1"

	record := model.ZoneRecord{
		ID:      recordID,
		ZoneID:  zoneID,
		Type:    "A",
		Name:    "www.example.com",
		Content: "10.0.0.1",
		TTL:     300,
	}
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecord", mock.Anything, activity.DeleteDNSRecordParams{
		DomainID: 42,
		Name:     "www.example.com",
		Type:     "A",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestDeleteDNSRecordFails_SetsStatusFailed() {
	recordID := "test-record-2"
	zoneID := "test-zone-2"

	record := model.ZoneRecord{
		ID:      recordID,
		ZoneID:  zoneID,
		Type:    "A",
		Name:    "www.example.com",
		Content: "10.0.0.1",
		TTL:     300,
	}
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(&record, nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecord", mock.Anything, mock.Anything).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteZoneRecordWorkflowTestSuite) TestGetRecordFails_SetsStatusFailed() {
	recordID := "test-record-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneRecordByID", mock.Anything, recordID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zone_records", ID: recordID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneRecordWorkflow, recordID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
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
