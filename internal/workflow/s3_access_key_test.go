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

// ---------- CreateS3AccessKeyWorkflow ----------

type CreateS3AccessKeyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateS3AccessKeyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateS3AccessKeyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateS3AccessKeyWorkflowTestSuite) TestSuccess() {
	keyID := "test-key-1"
	bucketID := "test-bucket-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.S3AccessKey{
		ID:              keyID,
		S3BucketID:      bucketID,
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Permissions:     "readwrite",
	}
	bucket := model.S3Bucket{
		ID:       bucketID,
		TenantID: &tenantID,
		Name:     "mybucket",
		ShardID:  &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_access_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetS3AccessKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateS3AccessKey", mock.Anything, activity.CreateS3AccessKeyParams{
		TenantID:        tenantID,
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_access_keys", ID: keyID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateS3AccessKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateS3AccessKeyWorkflowTestSuite) TestGetKeyFails_SetsStatusFailed() {
	keyID := "test-key-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_access_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetS3AccessKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("s3_access_keys", keyID)).Return(nil)
	s.env.ExecuteWorkflow(CreateS3AccessKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteS3AccessKeyWorkflow ----------

type DeleteS3AccessKeyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteS3AccessKeyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteS3AccessKeyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteS3AccessKeyWorkflowTestSuite) TestSuccess() {
	keyID := "test-key-1"
	bucketID := "test-bucket-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.S3AccessKey{
		ID:              keyID,
		S3BucketID:      bucketID,
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Permissions:     "readwrite",
	}
	bucket := model.S3Bucket{
		ID:       bucketID,
		TenantID: &tenantID,
		Name:     "mybucket",
		ShardID:  &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_access_keys", ID: keyID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetS3AccessKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteS3AccessKey", mock.Anything, activity.DeleteS3AccessKeyParams{
		TenantID:    tenantID,
		AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_access_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteS3AccessKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteS3AccessKeyWorkflowTestSuite) TestGetKeyFails_SetsStatusFailed() {
	keyID := "test-key-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_access_keys", ID: keyID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetS3AccessKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("s3_access_keys", keyID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteS3AccessKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateS3AccessKeyWorkflow(t *testing.T) {
	suite.Run(t, new(CreateS3AccessKeyWorkflowTestSuite))
}

func TestDeleteS3AccessKeyWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteS3AccessKeyWorkflowTestSuite))
}
