package request

type CreateCronJob struct {
	Name             string `json:"name" validate:"required,slug"`
	Schedule         string `json:"schedule" validate:"required"`
	Command          string `json:"command" validate:"required,max=4096"`
	WorkingDirectory string `json:"working_directory" validate:"omitempty,max=255"`
	TimeoutSeconds   int    `json:"timeout_seconds" validate:"omitempty,min=1,max=86400"`
	MaxMemoryMB      int    `json:"max_memory_mb" validate:"omitempty,min=16,max=4096"`
}

type UpdateCronJob struct {
	Schedule         *string `json:"schedule" validate:"omitempty"`
	Command          *string `json:"command" validate:"omitempty,max=4096"`
	WorkingDirectory *string `json:"working_directory" validate:"omitempty,max=255"`
	TimeoutSeconds   *int    `json:"timeout_seconds" validate:"omitempty,min=1,max=86400"`
	MaxMemoryMB      *int    `json:"max_memory_mb" validate:"omitempty,min=16,max=4096"`
}
