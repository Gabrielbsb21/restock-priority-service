package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// readinessTimeout bounds the dependency check behind /readyz, so a hung database
// answers 503 instead of holding the probe open.
const readinessTimeout = 2 * time.Second

type PartHandler struct {
	partService     *application.PartService
	priorityService *application.PriorityService
	readiness       application.ReadinessChecker
}

func NewPartHandler(
	partService *application.PartService,
	priorityService *application.PriorityService,
	readiness application.ReadinessChecker,
) *PartHandler {
	return &PartHandler{
		partService:     partService,
		priorityService: priorityService,
		readiness:       readiness,
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
		respondFromServiceError(c, err)
		return
	}

	c.Header("Location", "/parts/"+created.ID.String())
	c.JSON(http.StatusCreated, NewPartResponse(created))
}

func (h *PartHandler) GetPartByID(c *gin.Context) {
	id, ok := parsePartID(c)
	if !ok {
		return
	}

	part, err := h.partService.GetPartByID(c.Request.Context(), id)
	if err != nil {
		respondFromServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, NewPartResponse(part))
}

func (h *PartHandler) ListParts(c *gin.Context) {
	filter, ok := parseListFilter(c)
	if !ok {
		return
	}

	parts, total, err := h.partService.ListParts(c.Request.Context(), filter)
	if err != nil {
		respondFromServiceError(c, err)
		return
	}

	items := make([]PartResponse, 0, len(parts))
	for _, part := range parts {
		items = append(items, NewPartResponse(part))
	}

	c.JSON(http.StatusOK, ListPartsResponse{
		Items: items,
		Pagination: PaginationMeta{
			Limit:  filter.Limit,
			Offset: filter.Offset,
			Total:  total,
		},
	})
}

func (h *PartHandler) UpdatePart(c *gin.Context) {
	id, ok := parsePartID(c)
	if !ok {
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
	if err != nil {
		respondFromServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, NewPartResponse(updated))
}

func (h *PartHandler) DeletePart(c *gin.Context) {
	id, ok := parsePartID(c)
	if !ok {
		return
	}

	if err := h.partService.DeletePart(c.Request.Context(), id); err != nil {
		respondFromServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PartHandler) GetRestockPriorities(c *gin.Context) {
	priorities, err := h.priorityService.GetRestockPriorities(c.Request.Context())
	if err != nil {
		respondFromServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, NewPriorityListResponse(priorities))
}

func (h *PartHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *PartHandler) Readyz(c *gin.Context) {
	if h.readiness == nil {
		respondServiceUnavailable(c, "readiness dependency is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
	defer cancel()

	if err := h.readiness.CheckReadiness(ctx); err != nil {
		respondServiceUnavailable(c, "readiness dependency is unavailable")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// parsePartID reads the :id path parameter. It writes the error response itself and
// reports whether the caller should continue.
func parsePartID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondInvalidRequest(c, "invalid UUID path parameter")
		return uuid.Nil, false
	}

	return id, true
}

// parseListFilter validates the list query parameters. category matches exactly
// after trimming, and a present-but-empty value is invalid rather than ignored.
func parseListFilter(c *gin.Context) (application.ListFilter, bool) {
	var filter application.ListFilter

	if raw, present := c.GetQuery("category"); present {
		category := strings.TrimSpace(raw)
		if category == "" {
			respondInvalidRequest(c, "category must not be empty")
			return filter, false
		}
		filter.Category = category
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		respondInvalidRequest(c, "limit must be an integer between 1 and 100")
		return filter, false
	}
	filter.Limit = limit

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		respondInvalidRequest(c, "offset must be a non-negative integer")
		return filter, false
	}
	filter.Offset = offset

	return filter, true
}

// respondFromServiceError translates an application or domain error into the
// documented envelope. errors.As is used rather than a type assertion so wrapping a
// cause with %w does not silently downgrade a 400 into a 500.
func respondFromServiceError(c *gin.Context, err error) {
	var fieldErrs domain.FieldErrors
	if errors.As(err, &fieldErrs) {
		respondValidationError(c, fieldErrs)
		return
	}

	if errors.Is(err, application.ErrPartNotFound) {
		respondNotFound(c, "part with requested ID was not found")
		return
	}

	respondInternalError(c, err)
}
