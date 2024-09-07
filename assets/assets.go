package assets

import (
	"embed"
	"html/template"
)

var (
	//go:embed js css img
	Assets embed.FS
	//go:embed html
	templates embed.FS
)

func Templates() *template.Template {
	return template.Must(template.New("_all").ParseFS(templates, "html/*.html"))
}
