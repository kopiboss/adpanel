package main

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin/render"
)

// multiTemplateRenderer — satu template set per halaman.
// Menghindari conflict {{ define "content" }} yang sama antar halaman.
type multiTemplateRenderer struct {
	templates map[string]*template.Template
}

func (r *multiTemplateRenderer) Instance(name string, data interface{}) render.Render {
	tmpl, ok := r.templates[name]
	if !ok {
		panic("template not found: " + name)
	}
	return &templateRender{Template: tmpl, Name: name, Data: data}
}

type templateRender struct {
	Template *template.Template
	Name     string
	Data     interface{}
}

func (t *templateRender) Render(w http.ResponseWriter) error {
	t.WriteContentType(w)
	// Halaman dengan layout: entry point adalah "layout.html"
	// Auth pages (standalone): entry point adalah nama file itu sendiri
	entry := t.Name
	if t.Template.Lookup("layout.html") != nil {
		entry = "layout.html"
	}
	return t.Template.ExecuteTemplate(w, entry, t.Data)
}

func (t *templateRender) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}
