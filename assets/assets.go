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
		"dict":    dict,
		"rfc3339": func() string { return time.RFC3339 },
	}
)

func Templates() *template.Template {
	return template.Must(template.New("_all").Funcs(funcs).ParseFS(templates, "html/*.html"))
}

func dict(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("dict must have even number arguments, got: %d", len(kv))
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict argument %d must be string, got %T", i, kv[i])
		}
		m[k] = kv[i+1]
	}
	return m, nil
}
