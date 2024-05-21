package reporter

import (
	"bytes"
	"io/fs"
	"text/template"
)

func RenderLocalTemplate(path string, data any) (buf bytes.Buffer, err error) {
	return renderTemplate(nil, path, data)
}

func RenderEmbeddedTemplate(path string, data any) (buf bytes.Buffer, err error) {
	return renderTemplate(EmbeddedTemplatesFS, path, data)
}

func renderTemplate(fs fs.FS, path string, data any) (buf bytes.Buffer, err error) {
	var tmpl *template.Template

	if fs != nil {
		tmpl, err = template.ParseFS(fs, path)
	} else {
		tmpl, err = template.ParseFiles(path)
	}

	if err != nil {
		return buf, err
	}

	if err = tmpl.Execute(&buf, data); err != nil {
		return buf, err
	}

	return buf, nil
}
