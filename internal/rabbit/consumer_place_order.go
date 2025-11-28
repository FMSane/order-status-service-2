package rabbit

import (
	"context"
	"encoding/json"
	"log"

	"order-status-service-2/internal/dto"
	"order-status-service-2/internal/service"
)

type PlaceOrderConsumer struct {
	Service *service.OrderStatusService
}

func NewPlaceOrderConsumer(s *service.OrderStatusService) *PlaceOrderConsumer {
	return &PlaceOrderConsumer{Service: s}
}

// Se agrega el campo Shipping a la  estructura del mensaje para que el json.Unmarshal pueda capturarlo si existe.
type PlacedOrderMessage struct {
	CorrelationID string `json:"correlation_id"`
	Exchange      string `json:"exchange"`
	RoutingKey    string `json:"routing_key"`
	Message       struct {
		OrderID  string `json:"orderId"`
		CartID   string `json:"cartId"`
		UserID   string `json:"userId"`
		Articles []struct {
			ArticleID string `json:"articleId"`
			Quantity  int    `json:"quantity"`
		} `json:"articles"`
		// Agregamos esto. Si el JSON trae "shipping", se guarda aquí.
		// Si no lo trae, quedará vacío (Zero Value).
		Shipping dto.ShippingDTO `json:"shipping"`
	} `json:"message"`
}

func (c *PlaceOrderConsumer) Handle(msg []byte) error {

	log.Println("[Rabbit] Evento recibido: place_order")

	var event PlacedOrderMessage
	if err := json.Unmarshal(msg, &event); err != nil {
		log.Println("Error parseando mensaje:", err)
		return err
	}

	// Si Rabbit envió datos, van llenos.
	// Si Rabbit NO envió datos, va un struct vacío (AddressLine1 = "").
	// Si viene vacío, el servicio usará la dirección por defecto.
	_, err := c.Service.InitOrderStatus(
		context.Background(),
		event.Message.OrderID,
		event.Message.UserID,
		event.Message.Shipping,
		true,
	)

	if err != nil {
		log.Println("❌ Error creando estado inicial:", err)
		return err
	}

	log.Println("✔ Estado inicial procesado para orden:", event.Message.OrderID)
	return nil
}
