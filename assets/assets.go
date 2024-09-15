package assets

import (
	"embed"
	"fmt"
	"html/template"
	"time"
)

var (
	//go:embed js css img webfonts
	Assets embed.FS
	//go:embed html
	templates embed.FS

	funcs template.FuncMap = template.FuncMap{
		"errorf": func(format string, args ...any) (any, error) {
			return nil, fmt.Errorf(format, args...)
		},
		"formatTime": func(format string, t time.Time) string {
			return t.Format(format)
		},
	}
)

func Templates() *template.Template {
	return template.Must(template.New("_all").Funcs(funcs).ParseFS(templates, "html/*.html"))
}
