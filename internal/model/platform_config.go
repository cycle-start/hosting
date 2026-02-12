package model

type PlatformConfig struct {
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
}
