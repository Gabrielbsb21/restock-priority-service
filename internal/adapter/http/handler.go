package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PartHandler struct {
	partService     *application.PartService
	priorityService *application.PriorityService
	db              *gorm.DB
}

func NewPartHandler(partService *application.PartService, priorityService *application.PriorityService, db *gorm.DB) *PartHandler {
	return &PartHandler{
		partService:     partService,
		priorityService: priorityService,
		db:              db,
	}
}

func (h *PartHandler) CreatePart(c *gin.Context) {
	var req PartWriteRequest
	if err := BindJSONStrict(c, &req); err != nil {
		return
	}

	part, reqErrs := req.ToDomain()
	if len(reqErrs) > 0 {
		respondValidationError(c, reqErrs)
		return
	}

	created, err := h.partService.CreatePart(c.Request.Context(), part)
	if err != nil {
		if fieldErrs, ok := err.(domain.FieldErrors); ok {
			respondValidationError(c, fieldErrs)
			return
		}
		respondInternalError(c)
		return
	}

	c.Header("Location", "/parts/"+created.ID.String())
	c.JSON(http.StatusCreated, NewPartResponse(created))
}

func (h *PartHandler) GetPartByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondInvalidRequest(c, "invalid UUID path parameter")
		return
	}

	part, err := h.partService.GetPartByID(c.Request.Context(), id)
	if errors.Is(err, application.ErrPartNotFound) {
		respondNotFound(c, "part with requested ID was not found")
		return
	}
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, NewPartResponse(part))
}

func (h *PartHandler) ListParts(c *gin.Context) {
	category := c.Query("category")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		respondInvalidRequest(c, "limit must be an integer between 1 and 100")
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		respondInvalidRequest(c, "offset must be a non-negative integer")
		return
	}

	filter := application.ListFilter{
		Category: category,
		Limit:    limit,
		Offset:   offset,
	}

	parts, total, err := h.partService.ListParts(c.Request.Context(), filter)
	if err != nil {
		respondInternalError(c)
		return
	}

	items := make([]PartResponse, len(parts))
	for i, p := range parts {
		items[i] = NewPartResponse(p)
	}

	c.JSON(http.StatusOK, ListPartsResponse{
		Items: items,
		Pagination: PaginationMeta{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

func (h *PartHandler) UpdatePart(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondInvalidRequest(c, "invalid UUID path parameter")
		return
	}

	var req PartWriteRequest
	if err := BindJSONStrict(c, &req); err != nil {
		return
	}

	part, reqErrs := req.ToDomain()
	if len(reqErrs) > 0 {
		respondValidationError(c, reqErrs)
		return
	}

	updated, err := h.partService.UpdatePart(c.Request.Context(), id, part)
	if errors.Is(err, application.ErrPartNotFound) {
		respondNotFound(c, "part with requested ID was not found")
		return
	}
	if err != nil {
		if fieldErrs, ok := err.(domain.FieldErrors); ok {
			respondValidationError(c, fieldErrs)
			return
		}
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, NewPartResponse(updated))
}

func (h *PartHandler) DeletePart(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondInvalidRequest(c, "invalid UUID path parameter")
		return
	}

	err = h.partService.DeletePart(c.Request.Context(), id)
	if errors.Is(err, application.ErrPartNotFound) {
		respondNotFound(c, "part with requested ID was not found")
		return
	}
	if err != nil {
		respondInternalError(c)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PartHandler) GetRestockPriorities(c *gin.Context) {
	priorities, err := h.priorityService.GetRestockPriorities(c.Request.Context())
	if err != nil {
		respondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, PriorityListResponse{
		Priorities: priorities,
	})
}

func (h *PartHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *PartHandler) Readyz(c *gin.Context) {
	if h.db == nil {
		respondServiceUnavailable(c, "database not configured")
		return
	}

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		respondServiceUnavailable(c, "database connection failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
