package swagger

import (
	"net/http"

	"github.com/gin-gonic/gin"
	contract "github.com/wizzyszn/cooked/api"
)

const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Cooked API documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin="anonymous"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: '/docs/openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true,
        displayRequestDuration: true,
        persistAuthorization: true,
        tryItOutEnabled: true,
        presets: [SwaggerUIBundle.presets.apis],
        layout: 'BaseLayout'
      });
    };
  </script>
</body>
</html>`

func Register(router *gin.Engine) {
	router.GET("/docs", func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "/docs/") })
	router.GET("/docs/", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page)) })
	router.GET("/docs/openapi.yaml", func(c *gin.Context) { c.Data(http.StatusOK, "application/yaml; charset=utf-8", contract.OpenAPISpec) })
}
