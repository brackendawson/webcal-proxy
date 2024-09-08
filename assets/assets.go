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

	funcs template.FuncMap = template.FuncMap{
		"rem": func(x, y int) int {
			return x % y
		},
	}
)

func Templates() *template.Template {
	return template.Must(template.New("_all").Funcs(funcs).ParseFS(templates, "html/*.html"))
}
