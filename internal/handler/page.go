package handler

import (
	"html/template"
	"net/http"
)

// PageHandler serves the main dashboard HTML page.
type PageHandler struct {
	tmpl *template.Template
}

// NewPageHandler creates a new page handler with the embedded template.
func NewPageHandler(tmplStr string) (*PageHandler, error) {
	tmpl, err := template.New("index").Parse(tmplStr)
	if err != nil {
		return nil, err
	}
	return &PageHandler{tmpl: tmpl}, nil
}

// ServeHTTP handles GET / requests.
func (h *PageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if err := h.tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
