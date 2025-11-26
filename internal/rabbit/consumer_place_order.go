// consumer_place_order.go
package rabbit

import (
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

// Estructuras esperadas del JSON del evento
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
	} `json:"message"`
}

// Handler del evento
func (c *PlaceOrderConsumer) Handle(msg []byte) error {

	log.Println("===== RAW MENSAJE RECIBIDO =====")
	log.Println(string(msg))
	log.Println("================================")

	log.Println("[Rabbit] Evento recibido: place_order")

	var event PlacedOrderMessage
	if err := json.Unmarshal(msg, &event); err != nil {
		log.Println("Error parseando mensaje:", err)
		return err
	}

	// Construimos la request para inicializar estado
	req := dto.InitOrderStatusRequest{
		OrderID: event.Message.OrderID,
		UserID:  event.Message.UserID,
		Shipping: dto.ShippingDTO{
			AddressLine1: "Av San Martín 1234",
			City:         "Mendoza",
			PostalCode:   "5500",
			Province:     "Mendoza",
			Country:      "Argentina",
			Comments:     "Entregar cerca del mediodía",
		},
	}

	_, err := c.Service.InitOrderStatus(event.Message.OrderID, event.Message.UserID, req.Shipping, true)
	if err != nil {
		log.Println("❌ Error creando estado inicial:", err)
		return err
	}

	log.Println("✔ Estado inicial creado para orden:", event.Message.OrderID)

	return nil
}
