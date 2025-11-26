// dto.go
package dto

import "time"

// CreateOrderStatusRequest used by API + Rabbit consumer to create initial status
type InitOrderStatusRequest struct {
	OrderID  string      `json:"orderId" binding:"required"`
	UserID   string      `json:"userId" binding:"required"`
	Shipping ShippingDTO `json:"shipping"`
}

// ShippingDTO constant structure for addresses
type ShippingDTO struct {
	AddressLine1 string `json:"addressLine1"`
	City         string `json:"city"`
	PostalCode   string `json:"postalCode"`
	Province     string `json:"province"`
	Country      string `json:"country"`
	Comments     string `json:"comments"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

// Response DTOs
type OrderStatusResponse struct {
	OrderID   string      `json:"orderId"`
	UserID    string      `json:"userId"`
	Status    string      `json:"status"`
	Shipping  ShippingDTO `json:"shipping"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}
