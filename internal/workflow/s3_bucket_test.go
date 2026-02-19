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

// ---------- CreateS3BucketWorkflow ----------

type CreateS3BucketWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateS3BucketWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateS3BucketWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateS3BucketWorkflowTestSuite) TestSuccess() {
	bucketID := "test-bucket-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{ID: tenantID, Name: "ttest1234567"}
	bucket := model.S3Bucket{
		ID:         bucketID,
		TenantID:   &tenantID,
		Name:       "mybucket",
		ShardID:    &shardID,
		Public:     false,
		QuotaBytes: 1073741824,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateS3Bucket", mock.Anything, activity.CreateS3BucketParams{
		TenantID:   "ttest1234567",
		Name:       "ttest1234567-mybucket",
		QuotaBytes: 1073741824,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateS3BucketWorkflowTestSuite) TestSuccessPublicBucket() {
	bucketID := "test-bucket-pub"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{ID: tenantID, Name: "ttest1234567"}
	bucket := model.S3Bucket{
		ID:         bucketID,
		TenantID:   &tenantID,
		Name:       "publicbucket",
		ShardID:    &shardID,
		Public:     true,
		QuotaBytes: 1073741824,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	internalName := "ttest1234567-publicbucket"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateS3Bucket", mock.Anything, activity.CreateS3BucketParams{
		TenantID:   "ttest1234567",
		Name:       internalName,
		QuotaBytes: 1073741824,
	}).Return(nil)
	s.env.OnActivity("UpdateS3BucketPolicy", mock.Anything, activity.UpdateS3BucketPolicyParams{
		TenantID: "ttest1234567",
		Name:     internalName,
		Public:   true,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateS3BucketWorkflowTestSuite) TestGetBucketFails_SetsStatusFailed() {
	bucketID := "test-bucket-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("s3_buckets", bucketID)).Return(nil)
	s.env.ExecuteWorkflow(CreateS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateS3BucketWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	bucketID := "test-bucket-3"
	tenantID := "test-tenant-3"
	shardID := "test-shard-3"
	tenant := model.Tenant{ID: tenantID, Name: "ttest1234567"}
	bucket := model.S3Bucket{
		ID:         bucketID,
		TenantID:   &tenantID,
		Name:       "mybucket",
		ShardID:    &shardID,
		Public:     false,
		QuotaBytes: 1073741824,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateS3Bucket", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("s3_buckets", bucketID)).Return(nil)
	s.env.ExecuteWorkflow(CreateS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateS3BucketWorkflowTestSuite) TestSetProvisioningFails() {
	bucketID := "test-bucket-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))
	s.env.ExecuteWorkflow(CreateS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteS3BucketWorkflow ----------

type DeleteS3BucketWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteS3BucketWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteS3BucketWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteS3BucketWorkflowTestSuite) TestSuccess() {
	bucketID := "test-bucket-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{ID: tenantID, Name: "ttest1234567"}
	bucket := model.S3Bucket{
		ID:         bucketID,
		TenantID:   &tenantID,
		Name:       "mybucket",
		ShardID:    &shardID,
		Public:     false,
		QuotaBytes: 1073741824,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	internalName := "ttest1234567-mybucket"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteS3Bucket", mock.Anything, activity.DeleteS3BucketParams{
		TenantID: "ttest1234567",
		Name:     internalName,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteS3BucketWorkflowTestSuite) TestGetBucketFails_SetsStatusFailed() {
	bucketID := "test-bucket-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("s3_buckets", bucketID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteS3BucketWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	bucketID := "test-bucket-3"
	tenantID := "test-tenant-3"
	shardID := "test-shard-3"
	tenant := model.Tenant{ID: tenantID, Name: "ttest1234567"}
	bucket := model.S3Bucket{
		ID:         bucketID,
		TenantID:   &tenantID,
		Name:       "mybucket",
		ShardID:    &shardID,
		Public:     false,
		QuotaBytes: 1073741824,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "s3_buckets", ID: bucketID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetS3BucketByID", mock.Anything, bucketID).Return(&bucket, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteS3Bucket", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("s3_buckets", bucketID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteS3BucketWorkflow, bucketID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateS3BucketWorkflow(t *testing.T) {
	suite.Run(t, new(CreateS3BucketWorkflowTestSuite))
}

func TestDeleteS3BucketWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteS3BucketWorkflowTestSuite))
}
