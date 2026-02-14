package request

import (
	"encoding/json"
	"time"
)

// Nested types for CreateTenant
type CreateZoneNested struct {
	Name string `json:"name" validate:"required,fqdn"`
}

type CreateWebrootNested struct {
	Name           string             `json:"name" validate:"required,slug"`
	Runtime        string             `json:"runtime" validate:"required,oneof=php node python ruby static"`
	RuntimeVersion string             `json:"runtime_version" validate:"required"`
	RuntimeConfig  json.RawMessage    `json:"runtime_config"`
	PublicFolder   string             `json:"public_folder"`
	FQDNs          []CreateFQDNNested `json:"fqdns" validate:"omitempty,dive"`
}

type CreateFQDNNested struct {
	FQDN          string                     `json:"fqdn" validate:"required,fqdn"`
	SSLEnabled    *bool                      `json:"ssl_enabled"`
	EmailAccounts []CreateEmailAccountNested `json:"email_accounts" validate:"omitempty,dive"`
}

type CreateEmailAccountNested struct {
	Address     string                      `json:"address" validate:"required,email"`
	DisplayName string                      `json:"display_name"`
	QuotaBytes  int64                       `json:"quota_bytes"`
	Aliases     []CreateEmailAliasNested    `json:"aliases" validate:"omitempty,dive"`
	Forwards    []CreateEmailForwardNested  `json:"forwards" validate:"omitempty,dive"`
	AutoReply   *CreateEmailAutoReplyNested `json:"autoreply"`
}

type CreateEmailAliasNested struct {
	Address string `json:"address" validate:"required,email"`
}

type CreateEmailForwardNested struct {
	Destination string `json:"destination" validate:"required,email"`
	KeepCopy    *bool  `json:"keep_copy"`
}

type CreateEmailAutoReplyNested struct {
	Subject   string     `json:"subject" validate:"required"`
	Body      string     `json:"body" validate:"required"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Enabled   bool       `json:"enabled"`
}

type CreateDatabaseNested struct {
	Name    string                     `json:"name" validate:"required,mysql_name"`
	ShardID string                     `json:"shard_id" validate:"required"`
	Users   []CreateDatabaseUserNested `json:"users" validate:"omitempty,dive"`
}

type CreateDatabaseUserNested struct {
	Username   string   `json:"username" validate:"required,mysql_name"`
	Password   string   `json:"password" validate:"required,min=8"`
	Privileges []string `json:"privileges" validate:"required,min=1"`
}

type CreateValkeyInstanceNested struct {
	Name        string                   `json:"name" validate:"required,slug"`
	ShardID     string                   `json:"shard_id" validate:"required"`
	MaxMemoryMB int                      `json:"max_memory_mb" validate:"omitempty,min=1"`
	Users       []CreateValkeyUserNested `json:"users" validate:"omitempty,dive"`
}

type CreateValkeyUserNested struct {
	Username   string   `json:"username" validate:"required,slug"`
	Password   string   `json:"password" validate:"required,min=8"`
	Privileges []string `json:"privileges" validate:"required,min=1"`
	KeyPattern string   `json:"key_pattern"`
}

type CreateS3BucketNested struct {
	Name       string `json:"name" validate:"required,slug"`
	ShardID    string `json:"shard_id" validate:"required"`
	Public     *bool  `json:"public"`
	QuotaBytes *int64 `json:"quota_bytes"`
}

type CreateSSHKeyNested struct {
	Name      string `json:"name" validate:"required,min=1,max=255"`
	PublicKey string `json:"public_key" validate:"required"`
}
