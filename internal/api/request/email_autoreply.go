package request

import "time"

type UpdateEmailAutoReply struct {
	Subject   string     `json:"subject" validate:"required"`
	Body      string     `json:"body" validate:"required"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Enabled   bool       `json:"enabled"`
}
