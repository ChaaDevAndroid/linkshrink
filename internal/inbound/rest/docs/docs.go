package docs

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

//go:embed openapi.yaml
var spec []byte

const page = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>LinkShrink API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
  <style>
    html { background: #1b1b1f; }
    body { margin: 0; background: #1b1b1f; }
    .swagger-ui { filter: invert(88%) hue-rotate(180deg); }
    .swagger-ui .microlight, .swagger-ui .highlight-code { filter: invert(100%) hue-rotate(180deg); }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`

type runRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body"`
}

var allowedMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

// Register adds the docs routes. app must be the FULL handler (router + middleware),
// so the runner exercises exactly what a real client would hit.
func Register(mux *http.ServeMux, app http.Handler) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(spec)
	})
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	})
	mux.HandleFunc("POST /docs/run", run(app))
}

func run(app http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var in runRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "body must be JSON: {method, path, body}", http.StatusBadRequest)
			return
		}

		method := strings.ToUpper(strings.TrimSpace(in.Method))
		if !allowedMethods[method] {
			http.Error(w, "unsupported method", http.StatusBadRequest)
			return
		}

		u, err := url.ParseRequestURI(in.Path)
		if err != nil || u.Host != "" || !strings.HasPrefix(in.Path, "/") ||
			strings.ContainsAny(in.Path, " \t\r\n") {
			http.Error(w, "path must start with / and contain no spaces (example: /links)", http.StatusBadRequest)
			return
		}
		if strings.HasPrefix(u.Path, "/docs/run") {
			http.Error(w, "the runner cannot call itself", http.StatusBadRequest)
			return
		}

		req := httptest.NewRequest(method, in.Path, strings.NewReader(in.Body))
		req = req.WithContext(r.Context())
		if in.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}

		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)

		var b strings.Builder
		fmt.Fprintf(&b, "HTTP/1.1 %d %s\n", res.StatusCode, http.StatusText(res.StatusCode))
		_ = res.Header.Write(&b)
		b.WriteString("\n")
		b.Write(body)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(strings.ReplaceAll(b.String(), "\r\n", "\n")))
	}
}
