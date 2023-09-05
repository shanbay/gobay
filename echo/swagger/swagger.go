package swagger

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

type Opts func(*swaggerConfig)

func SetSwaggerHost(host string) Opts {
	return func(o *swaggerConfig) {
		o.SwaggerHost = host
	}
}

func SetSwaggerAuthorizer(f func(*http.Request) bool) Opts {
	return func(o *swaggerConfig) {
		o.Authorizer = f
	}
}

func SetSwaggerIsHTTPS(b bool) Opts {
	return func(o *swaggerConfig) {
		o.IsHTTPS = b
	}
}

// SwaggerOpts configures the SwaggerDoc middlewares
type swaggerConfig struct {
	// SpecURL the url to find the spec for
	SpecURL string
	// SwaggerHost for the js that generates the swagger ui site, defaults to: http://petstore.swagger.io/
	SwaggerHost string
	// When this return value is false, 403 will be responsed.
	Authorizer func(*http.Request) bool

	IsHTTPS bool
}

// SwaggerDoc creates a middleware to serve a documentation site for a swagger spec.
// This allows for altering the spec before starting the http listener.
func SwaggerDoc(basePath string, swaggerJson []byte, opts ...Opts) echo.MiddlewareFunc {
	config := &swaggerConfig{
		SpecURL:     path.Join(basePath, "swagger.json"),
		SwaggerHost: "https://petstore.swagger.io",
	}
	for _, opt := range opts {
		opt(config)
	}
	docPath := path.Join(basePath, "apidocs")

	// swagger html
	tmpl := template.Must(template.New("swaggerdoc").Parse(swaggerTemplateV2))
	buf := bytes.NewBuffer(nil)
	_ = tmpl.Execute(buf, config)
	uiHtml := buf.Bytes()

	// swagger json
	responseSwaggerJson := swaggerJson
	if config.IsHTTPS {
		responseSwaggerJson = []byte(strings.Replace(
			string(swaggerJson),
			`"schemes": [
    "http"
  ],`,
			`"schemes": [
    "https"
  ],`,
			1))
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if (path == docPath || path == config.SpecURL) && c.Request().Method == http.MethodGet {
				if config.Authorizer != nil {
					if !config.Authorizer(c.Request()) {
						return c.String(403, "Forbidden")
					}
				}
				if path == docPath {
					return c.HTML(200, string(uiHtml))
				} else {
					return c.JSONBlob(200, responseSwaggerJson)
				}
			}

			if next == nil {
				return c.String(404, fmt.Sprintf("%q not found", docPath))
			}
			return next(c)
		}
	}
}

const swaggerTemplateV2 = `
	<!-- HTML for static distribution bundle build -->
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <title>API documentation</title>
    <link rel="stylesheet" type="text/css" href="{{ .SwaggerHost }}/swagger-ui.css" >
    <link rel="icon" type="image/png" href="{{ .SwaggerHost }}/favicon-32x32.png" sizes="32x32" />
    <link rel="icon" type="image/png" href="{{ .SwaggerHost }}/favicon-16x16.png" sizes="16x16" />
    <style>
      html
      {
        box-sizing: border-box;
        overflow: -moz-scrollbars-vertical;
        overflow-y: scroll;
      }

      *,
      *:before,
      *:after
      {
        box-sizing: inherit;
      }

      body
      {
        margin:0;
        background: #fafafa;
      }
    </style>
  </head>

  <body>
    <div id="swagger-ui"></div>

    <script src="{{ .SwaggerHost }}/swagger-ui-bundle.js"> </script>
    <script src="{{ .SwaggerHost }}/swagger-ui-standalone-preset.js"> </script>
    <script>
    window.onload = function() {
      
      // Begin Swagger UI call region
      const ui = SwaggerUIBundle({
        "dom_id": "#swagger-ui",
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout",
        validatorUrl: "https://validator.swagger.io/validator",
        url: "{{ .SpecURL }}",
      })

      // End Swagger UI call region
      window.ui = ui
    }
  </script>
  </body>
</html>`
