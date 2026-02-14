package request

type CreateDatabaseUser struct {
	Username   string   `json:"username" validate:"required,mysql_name"`
	Password   string   `json:"password" validate:"required,min=8"`
	Privileges []string `json:"privileges" validate:"required,min=1"`
}

type UpdateDatabaseUser struct {
	Password   string   `json:"password" validate:"omitempty,min=8"`
	Privileges []string `json:"privileges" validate:"omitempty,min=1"`
}
