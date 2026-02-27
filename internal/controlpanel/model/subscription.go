package model

import "time"

type Subscription struct {
	ID                 string    `json:"id"`
	CustomerID         string    `json:"customer_id"`
	TenantID           string    `json:"tenant_id"`
	ProductName        string    `json:"product_name"`
	ProductDescription *string   `json:"product_description"`
	Modules            []string  `json:"modules"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"updated_at"`
}
