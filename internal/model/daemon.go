package model

import "time"

type Daemon struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	NodeID        *string           `json:"node_id,omitempty" db:"node_id"`
	WebrootID     string            `json:"webroot_id"`
	Command       string            `json:"command"`
	ProxyPath     *string           `json:"proxy_path,omitempty"`
	ProxyPort     *int              `json:"proxy_port,omitempty"`
	NumProcs      int               `json:"num_procs"`
	StopSignal    string            `json:"stop_signal"`
	StopWaitSecs  int               `json:"stop_wait_secs"`
	MaxMemoryMB  int    `json:"max_memory_mb"`
	Enabled      bool   `json:"enabled"`
	Status        string            `json:"status"`
	StatusMessage *string           `json:"status_message,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}
