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
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidTransition  = errors.New("transición de estado inválida")
	ErrFinalState         = errors.New("no se puede cambiar el estado de una orden en estado final")
	ErrOrderAlreadyExists = errors.New("la orden ya fue inicializada previamente")
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
func (s *OrderStatusService) InitOrderStatus(ctx context.Context, orderId string, userId string, shipping dto.ShippingDTO, fromRabbit bool) (*model.OrderStatus, error) {

	// 1. Primero preguntamos si ya existe
	existing, err := s.repo.FindByOrderID(ctx, orderId)

	// 2. Si NO hay error (significa que ya existe), no hacemos nada
	if err == nil && existing != nil {
		return nil, ErrOrderAlreadyExists
	}

	// 3. Si da error ErrNotFound, entonces sí la creamos desde cero

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

// UpdateStatus valida y realiza la transición entre estados según las reglas de negocio.
func (s *OrderStatusService) UpdateStatus(ctx context.Context, orderID string, newStatus string, reason string, actorID string, isAdmin bool) error {
	ord, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return err
	}

	current := ord.Status

	// Si el estado nuevo es el mismo que ya está, no hacemos nada
	if current == newStatus {
		return nil
	}
	// Si el estado actual es final, no se puede cambiar
	if finalStates[current] {
		return ErrFinalState
	}
	// Si el nuevo estado no es válido, error
	if !isValidState(newStatus) {
		return ErrInvalidTransition
	}

	// Determinamos si el actor es el dueño de la orden
	isOwner := ord.UserID == actorID

	// Puede realizar la transición si es admin?
	allowedAsAdmin := isAdmin && contains(adminTransitions[current], newStatus)

	// Puede realizar la transición si es dueño?
	allowedAsOwner := isOwner && contains(userTransitions[current], newStatus)

	// Tiene permiso para hacer cualquier cambio?
	if !isAdmin && !isOwner {
		return ErrForbidden // Ni es admin, ni es el dueño -> Fuera.
	}

	if !allowedAsAdmin && !allowedAsOwner {
		// Caso especial: Si es admin, pero no es el dueño, no puede cancelar
		if isAdmin && newStatus == "Cancelado" && !isOwner {
			return ErrForbidden
		}

		return ErrInvalidTransition
	}

	// Actualización del estado
	record := model.StatusRecord{
		Status:    newStatus,
		Reason:    reason,
		UserID:    actorID,
		Timestamp: time.Now(),
		Current:   true,
	}

	return s.repo.UpdateStatus(ctx, orderID, newStatus, record)
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}
