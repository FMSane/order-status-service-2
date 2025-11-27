// models.go
package model

import "time"

type OrderStatus struct {
	OrderID   string         `bson:"order_id" json:"orderId"`
	UserID    string         `bson:"user_id" json:"userId"`
	Status    string         `bson:"status" json:"status"` // estado actual
	History   []StatusRecord `bson:"history" json:"history"`
	Shipping  Shipping       `bson:"shipping" json:"shipping"`
	CreatedAt time.Time      `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time      `bson:"updated_at" json:"updatedAt"`
}

type Shipping struct {
	AddressLine1 string `bson:"address_line1" json:"addressLine1"`
	City         string `bson:"city" json:"city"`
	PostalCode   string `bson:"postal_code" json:"postalCode"`
	Province     string `bson:"province" json:"province"`
	Country      string `bson:"country" json:"country"`
	Comments     string `bson:"comments" json:"comments"`
}

type StatusRecord struct {
	Status    string    `bson:"status" json:"status"`
	Reason    string    `bson:"reason" json:"reason"`
	UserID    string    `bson:"user" json:"userId"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`

	// Para marcar cuál es el último
	Current bool `bson:"current" json:"current"`
}
