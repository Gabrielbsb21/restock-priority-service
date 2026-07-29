package http

import (
	"net/http"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/gin-gonic/gin"
)

// NewRouter wires the HTTP surface. It takes application ports only: nothing here
// knows which database is behind them.
func NewRouter(
	ginMode string,
	partService *application.PartService,
	priorityService *application.PriorityService,
	readiness application.ReadinessChecker,
) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()

	// gin.New() leaves this false, which makes the NoMethod handler below dead code
	// and turns a wrong method into a 404.
	r.HandleMethodNotAllowed = true

	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(RequestLogMiddleware())
	r.Use(MaxBodySizeMiddleware())

	handler := NewPartHandler(partService, priorityService, readiness)

	r.NoMethod(func(c *gin.Context) {
		respondWithError(c, http.StatusMethodNotAllowed, "method_not_allowed", "the requested path does not support the method", nil)
	})

	r.NoRoute(func(c *gin.Context) {
		respondRouteNotFound(c)
	})

	registerDocs(r)

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
