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

// ---------- CreateZoneWorkflow ----------

type CreateZoneWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateZoneWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateZoneWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateZoneWorkflowTestSuite) TestSuccess() {
	zoneID := "test-zone-1"
	zone := model.Zone{
		ID:   zoneID,
		Name: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "primary_ns").Return("ns1.example.com", nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "hostmaster_email").Return("hostmaster.example.com", nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "secondary_ns").Return("ns2.example.com", nil)
	s.env.OnActivity("WriteDNSZone", mock.Anything, activity.WriteDNSZoneParams{
		Name: "example.com",
		Type: "NATIVE",
	}).Return(42, nil)
	// SOA record
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "example.com",
		Type:     "SOA",
		Content:  "ns1.example.com hostmaster.example.com 1 10800 3600 604800 300",
		TTL:      86400,
	}).Return(nil)
	// Primary NS record
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "example.com",
		Type:     "NS",
		Content:  "ns1.example.com",
		TTL:      86400,
	}).Return(nil)
	// Secondary NS record
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "example.com",
		Type:     "NS",
		Content:  "ns2.example.com",
		TTL:      86400,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateZoneWorkflowTestSuite) TestGetZoneFails_SetsStatusFailed() {
	zoneID := "test-zone-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateZoneWorkflowTestSuite) TestWriteDNSZoneFails_SetsStatusFailed() {
	zoneID := "test-zone-3"
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "primary_ns").Return("ns1.example.com", nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "hostmaster_email").Return("hostmaster.example.com", nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "secondary_ns").Return("ns2.example.com", nil)
	s.env.OnActivity("WriteDNSZone", mock.Anything, activity.WriteDNSZoneParams{
		Name: "example.com",
		Type: "NATIVE",
	}).Return(0, fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateZoneWorkflowTestSuite) TestWriteSOAFails_SetsStatusFailed() {
	zoneID := "test-zone-4"
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "primary_ns").Return("ns1.example.com", nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "hostmaster_email").Return("hostmaster.example.com", nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "secondary_ns").Return("ns2.example.com", nil)
	s.env.OnActivity("WriteDNSZone", mock.Anything, mock.Anything).Return(42, nil)
	// SOA write fails
	s.env.OnActivity("WriteDNSRecord", mock.Anything, activity.WriteDNSRecordParams{
		DomainID: 42,
		Name:     "example.com",
		Type:     "SOA",
		Content:  "ns1.example.com hostmaster.example.com 1 10800 3600 604800 300",
		TTL:      86400,
	}).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateZoneWorkflowTestSuite) TestGetPlatformConfigFails_SetsStatusFailed() {
	zoneID := "test-zone-5"
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetPlatformConfig", mock.Anything, "primary_ns").Return("", fmt.Errorf("config not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteZoneWorkflow ----------

type DeleteZoneWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteZoneWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteZoneWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteZoneWorkflowTestSuite) TestSuccess() {
	zoneID := "test-zone-1"
	zone := model.Zone{
		ID:   zoneID,
		Name: "example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecordsByDomain", mock.Anything, 42).Return(nil)
	s.env.OnActivity("DeleteDNSZone", mock.Anything, "example.com").Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteZoneWorkflowTestSuite) TestGetZoneFails_SetsStatusFailed() {
	zoneID := "test-zone-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteZoneWorkflowTestSuite) TestDeleteDNSZoneFails_SetsStatusFailed() {
	zoneID := "test-zone-3"
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecordsByDomain", mock.Anything, 42).Return(nil)
	s.env.OnActivity("DeleteDNSZone", mock.Anything, "example.com").Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteZoneWorkflowTestSuite) TestDeleteRecordsByDomainFails_SetsStatusFailed() {
	zoneID := "test-zone-4"
	zone := model.Zone{ID: zoneID, Name: "example.com"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetZoneByID", mock.Anything, zoneID).Return(&zone, nil)
	s.env.OnActivity("GetDNSZoneIDByName", mock.Anything, "example.com").Return(42, nil)
	s.env.OnActivity("DeleteDNSRecordsByDomain", mock.Anything, 42).Return(fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "zones", ID: zoneID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteZoneWorkflow, zoneID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateZoneWorkflow(t *testing.T) {
	suite.Run(t, new(CreateZoneWorkflowTestSuite))
}

func TestDeleteZoneWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteZoneWorkflowTestSuite))
}
