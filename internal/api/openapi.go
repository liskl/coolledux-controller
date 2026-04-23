package api

import (
	_ "embed"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var openapiYAML []byte

// openapiJSON is produced once at init from openapiYAML so /openapi.json
// is a zero-allocation byte copy at request time.
var openapiJSON []byte

func init() {
	var doc any
	if err := yaml.Unmarshal(openapiYAML, &doc); err != nil {
		panic("openapi.yaml: malformed YAML: " + err.Error())
	}
	doc = normalizeForJSON(doc)
	b, err := json.Marshal(doc)
	if err != nil {
		panic("openapi.yaml: cannot encode as JSON: " + err.Error())
	}
	openapiJSON = b
}

// normalizeForJSON walks a value decoded by yaml.v3 and converts every
// map[any]any (which YAML produces for non-string-keyed subtrees) into a
// map[string]any. encoding/json rejects the former outright.
func normalizeForJSON(v any) any {
	switch x := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			ks, ok := k.(string)
			if !ok {
				ks = toString(k)
			}
			out[ks] = normalizeForJSON(val)
		}
		return out
	case map[string]any:
		for k, val := range x {
			x[k] = normalizeForJSON(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = normalizeForJSON(val)
		}
		return x
	default:
		return v
	}
}

func toString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// GetOpenAPIYAML serves the embedded OpenAPI 3.1 spec as YAML.
func (h *Handlers) GetOpenAPIYAML(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
	return c.Send(openapiYAML)
}

// GetOpenAPIJSON serves the embedded OpenAPI 3.1 spec as JSON.
func (h *Handlers) GetOpenAPIJSON(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "application/json; charset=utf-8")
	return c.Send(openapiJSON)
}

// docsHTML is a minimal Swagger UI page that loads /openapi.json. It pulls
// CSS/JS from unpkg.com; offline deployments should render the YAML with
// their own tooling.
const docsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>CoolLEDUX Controller API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>body{margin:0}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.addEventListener('load', function () {
      window.ui = SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
      });
    });
  </script>
</body>
</html>
`

// GetDocs serves a Swagger UI page that renders /openapi.json.
func (h *Handlers) GetDocs(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(docsHTML)
}
