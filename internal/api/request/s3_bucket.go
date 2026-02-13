package request

type CreateS3Bucket struct {
	Name       string `json:"name" validate:"required,slug"`
	ShardID    string `json:"shard_id" validate:"required"`
	Public     *bool  `json:"public"`
	QuotaBytes *int64 `json:"quota_bytes"`
}

type UpdateS3Bucket struct {
	Public     *bool  `json:"public"`
	QuotaBytes *int64 `json:"quota_bytes"`
}
