package repository

import (
	"context"
	"errors"
	"time"

	"order-status-service-2/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrNotFound = errors.New("orden no encontrada")

// Mongo implementation
type MongoOrderRepository struct {
	col *mongo.Collection
}

func NewMongoOrderRepository(db *mongo.Database) *MongoOrderRepository {
	return &MongoOrderRepository{col: db.Collection("order_statuses")}
}

func (m *MongoOrderRepository) Save(ctx context.Context, o *model.OrderStatus) error {
	now := time.Now().UTC()

	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
		// Primer estado en historial
		o.History = []model.StatusRecord{
			{
				Status:    o.Status,
				Timestamp: now,
				UserID:    o.UserID, // creador
				Reason:    "Orden creada",
				Current:   true,
			},
		}
	}
	o.UpdatedAt = now

	filter := bson.M{"order_id": o.OrderID}
	update := bson.M{"$set": o}
	opts := options.Update().SetUpsert(true)
	_, err := m.col.UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoOrderRepository) FindByOrderID(ctx context.Context, orderID string) (*model.OrderStatus, error) {
	var res model.OrderStatus
	err := m.col.FindOne(ctx, bson.M{"order_id": orderID}).Decode(&res)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	return &res, err
}

func (m *MongoOrderRepository) UpdateStatus(ctx context.Context, orderID, status string, record model.StatusRecord) error {

	// PASO 1: desmarcar el actual
	filter := bson.M{
		"order_id":        orderID,
		"history.current": true,
	}

	update1 := bson.M{
		"$set": bson.M{
			"history.$.current": false,
		},
	}

	r1, err := m.col.UpdateOne(ctx, filter, update1)
	if err != nil {
		return err
	}
	if r1.MatchedCount == 0 {
		return ErrNotFound
	}

	// PASO 2: actualizar estado + pushear nuevo registro
	filter2 := bson.M{"order_id": orderID}

	update2 := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now().UTC(),
		},
		"$push": bson.M{
			"history": record,
		},
	}

	_, err = m.col.UpdateOne(ctx, filter2, update2)
	return err
}

func (m *MongoOrderRepository) FindAll(ctx context.Context) ([]*model.OrderStatus, error) {
	cur, err := m.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []*model.OrderStatus
	for cur.Next(ctx) {
		var v model.OrderStatus
		if err := cur.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, &v)
	}
	return out, nil
}

func (m *MongoOrderRepository) FindByStatus(ctx context.Context, status string) ([]*model.OrderStatus, error) {
	cur, err := m.col.Find(ctx, bson.M{"status": status})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []*model.OrderStatus
	for cur.Next(ctx) {
		var v model.OrderStatus
		if err := cur.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, &v)
	}
	return out, nil
}

func (m *MongoOrderRepository) FindByUserID(ctx context.Context, userID string) ([]*model.OrderStatus, error) {
	cur, err := m.col.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []*model.OrderStatus
	for cur.Next(ctx) {
		var v model.OrderStatus
		if err := cur.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, &v)
	}
	return out, nil
}
