package http

import (
	"net/http"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(ginMode string, partService *application.PartService, priorityService *application.PriorityService, db *gorm.DB) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(MaxBodySizeMiddleware())

	handler := NewPartHandler(partService, priorityService, db)

	r.NoMethod(func(c *gin.Context) {
		respondWithError(c, http.StatusMethodNotAllowed, "method_not_allowed", "the requested path does not support the method", nil)
	})

	r.NoRoute(func(c *gin.Context) {
		respondNotFound(c, "resource not found")
	})

	r.GET("/healthz", handler.Healthz)
	r.GET("/readyz", handler.Readyz)

	parts := r.Group("/parts")
	{
		parts.POST("", handler.CreatePart)
		parts.GET("", handler.ListParts)
		parts.GET("/:id", handler.GetPartByID)
		parts.PUT("/:id", handler.UpdatePart)
		parts.DELETE("/:id", handler.DeletePart)
	}

	r.GET("/restock/priorities", handler.GetRestockPriorities)

	return r
}
