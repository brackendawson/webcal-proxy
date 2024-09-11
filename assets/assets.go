package assets

import (
	"embed"
	"fmt"
	"html/template"
)

var (
	//go:embed js css img
	Assets embed.FS
	//go:embed html
	templates embed.FS

	funcs template.FuncMap = template.FuncMap{
		"errorf": func(format string, args ...any) (any, error) {
			return nil, fmt.Errorf(format, args...)
		},
	}
)

func Templates() *template.Template {
	return template.Must(template.New("_all").Funcs(funcs).ParseFS(templates, "html/*.html"))
}
