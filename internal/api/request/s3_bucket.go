package request

type CreateS3Bucket struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	ShardID        string `json:"shard_id" validate:"required"`
	Public         *bool  `json:"public"`
	QuotaBytes     *int64 `json:"quota_bytes"`
}

type UpdateS3Bucket struct {
	Public     *bool  `json:"public"`
	QuotaBytes *int64 `json:"quota_bytes"`
}
