//go:build !dev

// see more in ./assets_dev.go
// about build flags in this project
package main

import (
	"embed"
	"html/template"
)

//go:embed templates/*.gohtml
var tmplFS embed.FS

func init() {
	galleryTempl = template.Must(template.New("gallery").Funcs(tmplFuncs).ParseFS(tmplFS, "templates/gallery.gohtml"))
	itemsTempl = template.Must(template.New("items").Funcs(tmplFuncs).ParseFS(tmplFS, "templates/gallery.gohtml"))
	detailTempl = template.Must(template.New("detail").Funcs(tmplFuncs).ParseFS(tmplFS, "templates/gallery.gohtml"))
}
