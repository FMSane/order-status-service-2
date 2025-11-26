// setup.go
package rabbit

import (
	"log"

	"order-status-service-2/internal/service"

	"github.com/rabbitmq/amqp091-go"
)

func SetupConsumers(ch *amqp091.Channel, svc *service.OrderStatusService) {
	consumer := NewPlaceOrderConsumer(svc)

	// 1. Declarar la queue
	q, err := ch.QueueDeclare(
		"order_status_service_orders", // cola exclusiva para tu micro
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println("❌ Error declarando queue:", err)
		return
	}

	// 2. Bindear al exchange fanout
	err = ch.QueueBind(
		q.Name,
		"",             // fanout ignora routing key
		"order_placed", // el exchange correcto
		false,
		nil,
	)
	if err != nil {
		log.Println("❌ Error binding exchange:", err)
		return
	}

	// 3. Consumir
	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println("❌ Error al consumir queue:", err)
		return
	}

	go func() {
		for m := range msgs {
			consumer.Handle(m.Body)
		}
	}()

	log.Println("🐰 Suscrito a exchange order_placed (fanout)")
}
