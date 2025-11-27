package controller

import (
	"net/http"
	"slices"

	"order-status-service-2/internal/dto"
	"order-status-service-2/internal/model"
	"order-status-service-2/internal/service"

	"github.com/gin-gonic/gin"
)

type OrderController struct {
	Service *service.OrderStatusService
}

func NewOrderController(s *service.OrderStatusService) *OrderController {
	return &OrderController{Service: s}
}

// POST /status/init — No requiere token
func (ctl *OrderController) InitStatus(c *gin.Context) {
	var req dto.InitOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := ctl.Service.InitOrderStatus(
		c.Request.Context(),
		req.OrderID,
		req.UserID,
		req.Shipping,
		false, // ← No viene desde Rabbit, viene desde la API
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

// PATCH /status/:orderId/status — requiere token
func (ctl *OrderController) UpdateStatus(c *gin.Context) {
	orderID := c.Param("orderId")

	var req dto.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actorID := c.GetString("userID")
	perms := c.GetStringSlice("userPermissions")
	isAdmin := slices.Contains(perms, "admin")

	err := ctl.Service.UpdateStatus(
		c.Request.Context(),
		orderID,
		req.Status,
		req.Reason,
		actorID,
		isAdmin,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "status updated"})
}

// GET /orders/mine - user (middleware debe poner userID)
func (ctl *OrderController) GetMyOrders(c *gin.Context) {
	userID := c.GetString("userID")
	orders, err := ctl.Service.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// GET /admin/orders - admin only (middleware AdminOnly)
func (ctl *OrderController) GetAllOrders(c *gin.Context) {
	orders, err := ctl.Service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// GET /admin/orders/state/:state - admin only
func (ctl *OrderController) GetAllOrdersByState(c *gin.Context) {
	state := c.Param("state")
	orders, err := ctl.Service.GetByStatus(c.Request.Context(), state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

func (ctl *OrderController) GetLatestStatus(c *gin.Context) {
	orderID := c.Param("orderId")
	actorID := c.GetString("userID")
	perms := c.GetStringSlice("userPermissions")
	isAdmin := slices.Contains(perms, "admin")

	// 1. Buscar orden
	o, err := ctl.Service.GetByOrderID(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	// 2. Validación de acceso
	if !isAdmin && o.UserID != actorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you cannot view another user's order"})
		return
	}

	// 3. Obtener estado actual
	var last *model.StatusRecord
	for _, h := range o.History {
		if h.Current {
			last = &h
			break
		}
	}

	if last == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no latest state found"})
		return
	}

	c.JSON(http.StatusOK, last)
}

func (ctl *OrderController) GetAllOrdersWithLatest(c *gin.Context) {
	orders, err := ctl.Service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var out []gin.H

	for _, o := range orders {
		var current string
		for _, h := range o.History {
			if h.Current {
				current = h.Status
				break
			}
		}

		out = append(out, gin.H{
			"orderId":  o.OrderID,
			"userId":   o.UserID,
			"status":   current,
			"shipping": o.Shipping,
		})
	}

	c.JSON(http.StatusOK, out)
}
