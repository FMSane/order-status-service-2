// config.go
package config

import "os"

type Config struct {
	MongoURI    string
	MongoDBName string
	AuthURL     string
	RabbitURL   string
	OrdersURL   string
	Port        string
}

func Load() *Config {
	return &Config{
		MongoURI:    getEnv("MONGO_URI", "mongodb://host.docker.internal:27017"),
		MongoDBName: getEnv("MONGO_DB_NAME", "order_status_db"),
		AuthURL:     getEnv("AUTH_URL", "http://host.docker.internal:3000"),
		RabbitURL:   getEnv("RABBIT_URL", "amqp://host.docker.internal"),
		OrdersURL:   getEnv("ORDERS_URL", "http://host.docker.internal:3004"),
		Port:        getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
