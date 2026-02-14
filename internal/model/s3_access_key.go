package model

import "time"

type S3AccessKey struct {
	ID              string    `json:"id" db:"id"`
	S3BucketID      string    `json:"s3_bucket_id" db:"s3_bucket_id"`
	AccessKeyID     string    `json:"access_key_id" db:"access_key_id"`
	SecretAccessKey string    `json:"secret_access_key,omitempty" db:"secret_access_key"`
	Permissions     string    `json:"permissions" db:"permissions"`
	Status          string    `json:"status" db:"status"`
	StatusMessage   *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}
