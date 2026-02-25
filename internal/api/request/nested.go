package request

import (
	"encoding/json"
	"time"
)

// Nested types for CreateTenant
type CreateSubscriptionNested struct {
	ID   string `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
}

type CreateZoneNested struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	Name           string `json:"name" validate:"required,fqdn"`
}

type CreateWebrootNested struct {
	SubscriptionID         string             `json:"subscription_id" validate:"required"`
	Runtime                string             `json:"runtime" validate:"required,oneof=php node python ruby static"`
	RuntimeVersion         string             `json:"runtime_version" validate:"required"`
	RuntimeConfig          json.RawMessage    `json:"runtime_config"`
	PublicFolder           string             `json:"public_folder"`
	EnvFileName            string             `json:"env_file_name"`
	ServiceHostnameEnabled *bool              `json:"service_hostname_enabled"`
	FQDNs                  []CreateFQDNNested `json:"fqdns" validate:"omitempty,dive"`
	Daemons                []CreateDaemonNested  `json:"daemons" validate:"omitempty,dive"`
	CronJobs               []CreateCronJobNested `json:"cron_jobs" validate:"omitempty,dive"`
}

type CreateFQDNNested struct {
	FQDN          string                     `json:"fqdn" validate:"required,fqdn"`
	SSLEnabled    *bool                      `json:"ssl_enabled"`
	EmailAccounts []CreateEmailAccountNested `json:"email_accounts" validate:"omitempty,dive"`
}

type CreateEmailAccountNested struct {
	SubscriptionID string                      `json:"subscription_id" validate:"required"`
	Address        string                      `json:"address" validate:"required,email"`
	DisplayName    string                      `json:"display_name"`
	QuotaBytes     int64                       `json:"quota_bytes"`
	Aliases        []CreateEmailAliasNested    `json:"aliases" validate:"omitempty,dive"`
	Forwards       []CreateEmailForwardNested  `json:"forwards" validate:"omitempty,dive"`
	AutoReply      *CreateEmailAutoReplyNested `json:"autoreply"`
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
	SubscriptionID string                           `json:"subscription_id" validate:"required"`
	ShardID        string                           `json:"shard_id" validate:"required"`
	Users          []CreateDatabaseUserNested       `json:"users" validate:"omitempty,dive"`
	AccessRules    []CreateDatabaseAccessRuleNested `json:"access_rules" validate:"omitempty,dive"`
}

type CreateDatabaseUserNested struct {
	Username   string   `json:"username" validate:"required,mysql_name"`
	Password   string   `json:"password" validate:"required,min=8"`
	Privileges []string `json:"privileges" validate:"required,min=1"`
}

type CreateValkeyInstanceNested struct {
	SubscriptionID string                   `json:"subscription_id" validate:"required"`
	ShardID        string                   `json:"shard_id" validate:"required"`
	MaxMemoryMB    int                      `json:"max_memory_mb" validate:"omitempty,min=1"`
	Users          []CreateValkeyUserNested `json:"users" validate:"omitempty,dive"`
}

type CreateValkeyUserNested struct {
	Username   string   `json:"username" validate:"required,slug"`
	Password   string   `json:"password" validate:"required,min=8"`
	Privileges []string `json:"privileges" validate:"required,min=1"`
	KeyPattern string   `json:"key_pattern"`
}

type CreateS3BucketNested struct {
	SubscriptionID string                    `json:"subscription_id" validate:"required"`
	ShardID        string                    `json:"shard_id" validate:"required"`
	Public         *bool                     `json:"public"`
	QuotaBytes     *int64                    `json:"quota_bytes"`
	AccessKeys     []CreateS3AccessKeyNested `json:"access_keys" validate:"omitempty,dive"`
}

type CreateSSHKeyNested struct {
	Name      string `json:"name" validate:"required,min=1,max=255"`
	PublicKey string `json:"public_key" validate:"required"`
}

type CreateEgressRuleNested struct {
	CIDR        string `json:"cidr" validate:"required"`
	Description string `json:"description"`
}

type CreateDatabaseAccessRuleNested struct {
	CIDR        string `json:"cidr" validate:"required"`
	Description string `json:"description"`
}

type CreateS3AccessKeyNested struct{}

type CreateDaemonNested struct {
	Command   string `json:"command" validate:"required"`
	ProxyPath string `json:"proxy_path"`
	NumProcs  int    `json:"num_procs"`
	ProxyPort int    `json:"proxy_port"`
}

type CreateCronJobNested struct {
	Schedule         string `json:"schedule" validate:"required"`
	Command          string `json:"command" validate:"required"`
	WorkingDirectory string `json:"working_directory"`
}
