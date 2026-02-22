package request

type CreateDaemon struct {
	Command      string            `json:"command" validate:"required,max=4096"`
	ProxyPath    *string           `json:"proxy_path" validate:"omitempty,startswith=/,max=255"`
	NumProcs     int               `json:"num_procs" validate:"omitempty,min=1,max=8"`
	StopSignal   string            `json:"stop_signal" validate:"omitempty,oneof=TERM INT QUIT KILL HUP"`
	StopWaitSecs int               `json:"stop_wait_secs" validate:"omitempty,min=1,max=300"`
	MaxMemoryMB  int    `json:"max_memory_mb" validate:"omitempty,min=16,max=4096"`
}

type UpdateDaemon struct {
	Command      *string `json:"command" validate:"omitempty,max=4096"`
	ProxyPath    *string `json:"proxy_path" validate:"omitempty,max=255"`
	NumProcs     *int    `json:"num_procs" validate:"omitempty,min=1,max=8"`
	StopSignal   *string `json:"stop_signal" validate:"omitempty,oneof=TERM INT QUIT KILL HUP"`
	StopWaitSecs *int    `json:"stop_wait_secs" validate:"omitempty,min=1,max=300"`
	MaxMemoryMB  *int    `json:"max_memory_mb" validate:"omitempty,min=16,max=4096"`
}
