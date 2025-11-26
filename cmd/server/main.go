package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"order-status-service-2/internal/config"
	"order-status-service-2/internal/controller"
	"order-status-service-2/internal/middleware"
	"order-status-service-2/internal/rabbit"
	"order-status-service-2/internal/repository"
	"order-status-service-2/internal/service"
)

func main() {
	cfg := config.Load()

	// Conexión a MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal(err)
	}
	db := client.Database(cfg.MongoDBName)

	// Repositorio y servicios
	repo := repository.NewMongoOrderRepository(db)
	orderService := service.NewOrderStatusService(repo)
	authService := service.NewAuthService()

	// Handlers
	ctrl := controller.NewOrderController(orderService)

	// Router
	r := gin.Default()

	// Rutas públicas
	r.POST("/status/init", ctrl.InitStatus)

	// Rutas protegidas (requieren token)
	auth := r.Group("/")
	auth.Use(middleware.AuthMiddleware(authService))

	auth.PATCH("/orders/:orderId/status", ctrl.UpdateStatus)
	auth.GET("/orders/mine", ctrl.GetMyOrders)
	auth.GET("/orders/:orderId/latest", ctrl.GetLatestStatus)

	// Rutas admin
	admin := auth.Group("/admin")
	admin.Use(middleware.AdminOnly())
	admin.GET("/orders/all", ctrl.GetAllOrders)
	admin.GET("/orders/:state", ctrl.GetAllOrdersByState)
	admin.GET("/orders-with-status", ctrl.GetAllOrdersWithLatest)

	// Conexión a RabbitMQ
	conn, err := amqp091.Dial(cfg.RabbitURL)
	if err != nil {
		log.Fatalf("Error conectando a RabbitMQ: %v", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Error creando canal en RabbitMQ: %v", err)
	}

	rabbit.SetupConsumers(ch, orderService)

	// Ejecutar servidor
	log.Printf("Order Status Service ejecutándose en puerto %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
