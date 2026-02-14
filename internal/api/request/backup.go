package request

type CreateBackup struct {
	Type     string `json:"type" validate:"required,oneof=web database"`
	SourceID string `json:"source_id" validate:"required"`
}

type RestoreBackup struct {
	// empty for now -- restores the whole backup
}
