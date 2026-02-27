package model

import "time"

type Customer struct {
	ID        string    `json:"id"`
	PartnerID string    `json:"partner_id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CustomerUser struct {
	CustomerID  string   `json:"customer_id"`
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions"`
}
