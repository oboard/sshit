package web

import "embed"

// Dist contains the built web frontend.
//
//go:embed dist/* dist/assets/*
var Dist embed.FS
