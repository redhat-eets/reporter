package reporter

import (
	"embed"
)

//go:embed config.yaml.default
var EmbeddedDefaultConfig []byte

//go:embed templates/*
var EmbeddedTemplatesFS embed.FS
