package service

import (
	"context"
	"errors"
	"time"

	"order-status-service-2/internal/dto"
	"order-status-service-2/internal/model"
)

// Interfaz que debe implementar repository
type OrderRepository interface {
	Save(ctx context.Context, o *model.OrderStatus) error
	FindByOrderID(ctx context.Context, orderID string) (*model.OrderStatus, error)
	UpdateStatus(ctx context.Context, orderID, status string, record model.StatusRecord) error
	FindAll(ctx context.Context) ([]*model.OrderStatus, error)
	FindByStatus(ctx context.Context, status string) ([]*model.OrderStatus, error)
	FindByUserID(ctx context.Context, userID string) ([]*model.OrderStatus, error)
}

func dtoToModelShipping(in dto.ShippingDTO) model.Shipping {
	return model.Shipping{
		AddressLine1: in.AddressLine1,
		City:         in.City,
		PostalCode:   in.PostalCode,
		Province:     in.Province,
		Country:      in.Country,
		Comments:     in.Comments,
	}
}

// Errores de negocio exportados (los usa el controller)
var (
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrFinalState        = errors.New("cannot change final state")
)

type OrderStatusService struct {
	repo OrderRepository
}

func NewOrderStatusService(r OrderRepository) *OrderStatusService {
	return &OrderStatusService{repo: r}
}

// Estados válidos (por nombre). No hay catálogo en BD.
var validStates = map[string]bool{
	"Pendiente":      true,
	"En Preparación": true,
	"Enviado":        true,
	"Entregado":      true,
	"Cancelado":      true,
	"Rechazado":      true,
}

func isValidState(s string) bool {
	return validStates[s]
}

// Transiciones permitidas (hardcodeadas por nombre) para admin y user
var adminTransitions = map[string][]string{
	"Pendiente":      {"En Preparación", "Rechazado"},
	"En Preparación": {"Enviado", "Rechazado"},
	"Enviado":        {"Entregado"},
}

var userTransitions = map[string][]string{
	"Pendiente":      {"Cancelado"},
	"En Preparación": {"Cancelado"},
}

// Estados finales
var finalStates = map[string]bool{
	"Cancelado": true,
	"Rechazado": true,
	"Entregado": true,
}

// CreateStatus crea o hace upsert del estado inicial de la orden.
// IMPORTANTE: fuerza el estado a "Pendiente" (siempre).
// Se puede invocar desde el consumer Rabbit (primario) o vía API para pruebas.
// Si el shipping del request está vacío, se usa la dirección constante.
func (s *OrderStatusService) InitOrderStatus(orderId string, userId string, shipping dto.ShippingDTO, fromRabbit bool) (*model.OrderStatus, error) {

	// Shipping por defecto si viene vacío
	if shipping.AddressLine1 == "" {
		shipping = dto.ShippingDTO{
			AddressLine1: "Av San Martín 1234",
			City:         "Mendoza",
			PostalCode:   "5500",
			Province:     "Mendoza",
			Country:      "Argentina",
			Comments:     "Orden inicializada automáticamente",
		}
	}

	status := &model.OrderStatus{
		OrderID:   orderId,
		UserID:    userId,
		Status:    "Pendiente",
		Shipping:  dtoToModelShipping(shipping),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		History: []model.StatusRecord{
			{
				Status:    "Pendiente",
				Current:   true,
				Reason:    "Orden inicializada",
				UserID:    userId,
				Timestamp: time.Now(),
			},
		},
	}

	return status, s.repo.Save(context.Background(), status)
}

// Getters
func (s *OrderStatusService) GetByOrderID(ctx context.Context, orderID string) (*model.OrderStatus, error) {
	return s.repo.FindByOrderID(ctx, orderID)
}

func (s *OrderStatusService) GetAll(ctx context.Context) ([]*model.OrderStatus, error) {
	return s.repo.FindAll(ctx)
}

func (s *OrderStatusService) GetByStatus(ctx context.Context, status string) ([]*model.OrderStatus, error) {
	return s.repo.FindByStatus(ctx, status)
}

func (s *OrderStatusService) GetByUserID(ctx context.Context, userID string) ([]*model.OrderStatus, error) {
	return s.repo.FindByUserID(ctx, userID)
}

// UpdateStatus valida y realiza la transición según rol (isAdmin).
// actorID es el id de quien realiza la acción (provisto por el middleware).
func (s *OrderStatusService) UpdateStatus(ctx context.Context, orderID string, newStatus string, reason string, actorID string, isAdmin bool) error {
	ord, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return err
	}

	current := ord.Status

	// Si es el mismo estado -> no hacer nada
	if current == newStatus {
		return nil
	}

	// Si actual ya es final -> bloquear
	if finalStates[current] {
		return ErrFinalState
	}

	// Validar que el nuevo estado exista
	if !isValidState(newStatus) {
		return ErrInvalidTransition
	}

	// Reglas ADMIN
	if isAdmin {
		// Admin no puede establecer Cancelado
		if newStatus == "Cancelado" {
			return ErrForbidden
		}
		// Rechazar no permitido si current está en Cancelado, Enviado, Entregado
		if newStatus == "Rechazado" {
			if current == "Cancelado" || current == "Enviado" || current == "Entregado" {
				return ErrInvalidTransition
			}
		}
		allowed := adminTransitions[current]
		if contains(allowed, newStatus) {
			record := model.StatusRecord{
				Status:    newStatus,
				Reason:    reason,
				UserID:    actorID,
				Timestamp: time.Now(),
				Current:   true,
			}
			return s.repo.UpdateStatus(ctx, orderID, newStatus, record)

		}
		return ErrInvalidTransition
	}

	// Reglas USER
	// User solo puede actuar sobre sus propias órdenes
	if ord.UserID != actorID {
		return ErrForbidden
	}

	// User puede cancelar solo si current no es Enviado/Entregado/Rechazado
	if newStatus == "Cancelado" {
		if current == "Enviado" || current == "Entregado" || current == "Rechazado" {
			return ErrInvalidTransition
		}
		record := model.StatusRecord{
			Status:    newStatus,
			Reason:    reason,
			UserID:    actorID,
			Timestamp: time.Now(),
			Current:   true,
		}
		return s.repo.UpdateStatus(ctx, orderID, newStatus, record)

	}
	allowed := userTransitions[current]
	if contains(allowed, newStatus) {
		record := model.StatusRecord{
			Status:    newStatus,
			Reason:    reason,
			UserID:    actorID,
			Timestamp: time.Now(),
			Current:   true,
		}
		return s.repo.UpdateStatus(ctx, orderID, newStatus, record)

	}
	return ErrInvalidTransition
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}
