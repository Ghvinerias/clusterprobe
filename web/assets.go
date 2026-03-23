package web

import "embed"

// FS provides embedded access to UI templates and static assets.
//
//go:embed templates/** static/**
var FS embed.FS
