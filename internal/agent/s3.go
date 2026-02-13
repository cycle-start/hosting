package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/zerolog"
)

// S3Manager handles S3 object storage operations via radosgw-admin CLI
// and the S3 API (via AWS SDK) against a local Ceph RGW endpoint.
type S3Manager struct {
	logger      zerolog.Logger
	endpoint    string // local RGW endpoint, e.g. "http://localhost:7480"
	adminKey    string // RGW admin access key
	adminSecret string // RGW admin secret key
}

// NewS3Manager creates a new S3Manager.
func NewS3Manager(logger zerolog.Logger, endpoint, adminKey, adminSecret string) *S3Manager {
	return &S3Manager{
		logger:      logger.With().Str("component", "s3-manager").Logger(),
		endpoint:    endpoint,
		adminKey:    adminKey,
		adminSecret: adminSecret,
	}
}

// s3Client returns an S3 client configured for the local RGW endpoint.
func (m *S3Manager) s3Client() *s3.Client {
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(m.endpoint),
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider(m.adminKey, m.adminSecret, ""),
		UsePathStyle: true,
	})
}

// execRGWAdmin runs a radosgw-admin command and returns the combined output.
func (m *S3Manager) execRGWAdmin(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "radosgw-admin", args...)
	m.logger.Debug().Strs("args", args).Msg("executing radosgw-admin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("radosgw-admin %s failed: %w: %s", args[0], err, string(output))
	}
	return output, nil
}

// ensureUser ensures the RGW user exists, creating it if needed.
func (m *S3Manager) ensureUser(ctx context.Context, tenantID string) error {
	_, err := m.execRGWAdmin(ctx, "user", "info", "--uid="+tenantID)
	if err != nil {
		// User doesn't exist, create it.
		_, err = m.execRGWAdmin(ctx, "user", "create",
			"--uid="+tenantID,
			"--display-name="+tenantID,
		)
		if err != nil {
			return fmt.Errorf("create RGW user %s: %w", tenantID, err)
		}
		m.logger.Info().Str("uid", tenantID).Msg("created RGW user")
	}
	return nil
}

// CreateBucket creates an S3 bucket via the S3 API and links it to the tenant user.
func (m *S3Manager) CreateBucket(ctx context.Context, tenantID, name string, quotaBytes int64) error {
	m.logger.Info().Str("tenant", tenantID).Str("bucket", name).Msg("creating S3 bucket")

	// Ensure RGW user exists.
	if err := m.ensureUser(ctx, tenantID); err != nil {
		return err
	}

	// Create bucket via S3 API using admin credentials.
	// The admin user retains ownership so it can manage policies.
	// Tenant access is granted via per-user access keys + bucket policy.
	client := m.s3Client()
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		// If bucket already exists, that's fine.
		if !strings.Contains(err.Error(), "BucketAlreadyExists") &&
			!strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") {
			return fmt.Errorf("create bucket %s: %w", name, err)
		}
	}

	// Grant the tenant user full access to the bucket via a bucket policy.
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":       "TenantFullAccess",
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": []string{fmt.Sprintf("arn:aws:iam:::user/%s", tenantID)}},
				"Action":    "s3:*",
				"Resource": []string{
					fmt.Sprintf("arn:aws:s3:::%s", name),
					fmt.Sprintf("arn:aws:s3:::%s/*", name),
				},
			},
		},
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal tenant policy: %w", err)
	}
	_, err = client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(name),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return fmt.Errorf("set tenant bucket policy: %w", err)
	}

	// Set quota if non-zero.
	if quotaBytes > 0 {
		if err := m.setBucketQuota(ctx, tenantID, name, quotaBytes); err != nil {
			return err
		}
	}

	return nil
}

// setBucketQuota sets a size quota on a bucket.
func (m *S3Manager) setBucketQuota(ctx context.Context, tenantID, name string, bytes int64) error {
	_, err := m.execRGWAdmin(ctx, "quota", "set",
		"--uid="+tenantID,
		"--bucket="+name,
		fmt.Sprintf("--max-size=%d", bytes),
	)
	if err != nil {
		return fmt.Errorf("set bucket quota: %w", err)
	}
	_, err = m.execRGWAdmin(ctx, "quota", "enable",
		"--uid="+tenantID,
		"--bucket="+name,
		"--quota-scope=bucket",
	)
	if err != nil {
		return fmt.Errorf("enable bucket quota: %w", err)
	}
	return nil
}

// DeleteBucket removes an S3 bucket and all its objects.
func (m *S3Manager) DeleteBucket(ctx context.Context, tenantID, name string) error {
	m.logger.Info().Str("tenant", tenantID).Str("bucket", name).Msg("deleting S3 bucket")

	// First, delete all objects in the bucket via S3 API.
	client := m.s3Client()
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(name),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			break // Bucket may already be empty or gone.
		}
		if len(page.Contents) == 0 {
			break
		}
		objects := make([]s3types.ObjectIdentifier, len(page.Contents))
		for i, obj := range page.Contents {
			objects[i] = s3types.ObjectIdentifier{Key: obj.Key}
		}
		_, _ = client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(name),
			Delete: &s3types.Delete{Objects: objects},
		})
	}

	// Delete the bucket itself.
	_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil && !strings.Contains(err.Error(), "NoSuchBucket") {
		return fmt.Errorf("delete bucket %s: %w", name, err)
	}

	return nil
}

// CreateAccessKey creates an S3 access key for the RGW user.
func (m *S3Manager) CreateAccessKey(ctx context.Context, tenantID, accessKey, secretKey string) error {
	m.logger.Info().Str("tenant", tenantID).Str("access_key", accessKey).Msg("creating S3 access key")

	if err := m.ensureUser(ctx, tenantID); err != nil {
		return err
	}

	_, err := m.execRGWAdmin(ctx, "key", "create",
		"--uid="+tenantID,
		"--access-key="+accessKey,
		"--secret-key="+secretKey,
		"--key-type=s3",
	)
	if err != nil {
		return fmt.Errorf("create access key for %s: %w", tenantID, err)
	}

	return nil
}

// DeleteAccessKey removes an S3 access key from the RGW user.
func (m *S3Manager) DeleteAccessKey(ctx context.Context, tenantID, accessKey string) error {
	m.logger.Info().Str("tenant", tenantID).Str("access_key", accessKey).Msg("deleting S3 access key")

	_, err := m.execRGWAdmin(ctx, "key", "rm",
		"--uid="+tenantID,
		"--access-key="+accessKey,
	)
	if err != nil {
		return fmt.Errorf("delete access key %s: %w", accessKey, err)
	}

	return nil
}

// SetBucketPolicy sets the bucket policy, always preserving tenant access and
// optionally adding public read access.
func (m *S3Manager) SetBucketPolicy(ctx context.Context, tenantID, name string, public bool) error {
	m.logger.Info().Str("bucket", name).Bool("public", public).Msg("setting S3 bucket policy")

	client := m.s3Client()

	// Always include tenant full access.
	statements := []map[string]any{
		{
			"Sid":       "TenantFullAccess",
			"Effect":    "Allow",
			"Principal": map[string]any{"AWS": []string{fmt.Sprintf("arn:aws:iam:::user/%s", tenantID)}},
			"Action":    "s3:*",
			"Resource": []string{
				fmt.Sprintf("arn:aws:s3:::%s", name),
				fmt.Sprintf("arn:aws:s3:::%s/*", name),
			},
		},
	}

	if public {
		statements = append(statements, map[string]any{
			"Sid":       "PublicRead",
			"Effect":    "Allow",
			"Principal": "*",
			"Action":    "s3:GetObject",
			"Resource":  fmt.Sprintf("arn:aws:s3:::%s/*", name),
		})
	}

	policy := map[string]any{
		"Version":   "2012-10-17",
		"Statement": statements,
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal bucket policy: %w", err)
	}

	_, err = client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(name),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}

	return nil
}
