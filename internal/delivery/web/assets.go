package web

import "embed"

//go:embed templates/*.tmpl static/*
var embeddedAssets embed.FS

