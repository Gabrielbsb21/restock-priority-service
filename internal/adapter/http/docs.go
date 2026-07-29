package http

import (
	"net/http"
	"strings"

	"github.com/Gabrielbsb21/restock-priority-service/api"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files/v2"
)

const (
	// openAPIPath serves the machine-readable contract.
	openAPIPath = "/openapi.yaml"
	// docsPath serves the browsable Swagger UI for that contract.
	docsPath = "/docs"

	initializerFile = "swagger-initializer.js"
)

// swaggerInitializer replaces the one shipped with Swagger UI, which points the page at
// the public petstore demo. Serving the assets without this override would document
// somebody else's API.
const swaggerInitializer = `window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: "` + openAPIPath + `",
    dom_id: "#swagger-ui",
    deepLinking: true,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    plugins: [SwaggerUIBundle.plugins.DownloadUrl],
    layout: "StandaloneLayout",
    tryItOutEnabled: true,
  });
};
`

// registerDocs serves the OpenAPI document and a Swagger UI for it.
//
// Both the document and the UI assets are embedded in the binary, so the documentation
// needs no network access, no CDN and no files on disk. The endpoints are unauthenticated
// like the rest of v1, and describe only a contract that is already public.
func registerDocs(r *gin.Engine) {
	r.GET(openAPIPath, func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", api.OpenAPISpec)
	})

	// Swagger UI resolves its assets relative to the directory it was loaded from, so
	// the bare path has to become a directory path.
	r.GET(docsPath, func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, docsPath+"/")
	})

	assets := http.StripPrefix(docsPath, http.FileServer(http.FS(swaggerFiles.FS)))

	r.GET(docsPath+"/*filepath", func(c *gin.Context) {
		if strings.TrimPrefix(c.Param("filepath"), "/") == initializerFile {
			c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(swaggerInitializer))
			return
		}

		assets.ServeHTTP(c.Writer, c.Request)
	})
}
