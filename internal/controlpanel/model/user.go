package model

import "time"

type User struct {
	ID             string    `json:"id"`
	PartnerID      string    `json:"partner_id"`
	Email          string    `json:"email"`
	PasswordHash   string    `json:"-"`
	DisplayName    *string   `json:"display_name"`
	Locale         string    `json:"locale"`
	LastCustomerID *string   `json:"last_customer_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
