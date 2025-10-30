// Package web embeds the HTML templates and static assets for the pellets tracker UI.
package web

import "embed"

// Assets exposes embedded templates and static files for the web UI.
//
//go:embed templates/*.tmpl static/*
var Assets embed.FS
