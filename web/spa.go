package web

import "embed"

// Dist holds the built SPA assets. The directory is populated by
// `bun run build` before the Go binary is compiled. A committed stub
// index.html keeps the embed valid without a frontend build so that
// `go test ./...` works from a fresh checkout.
//
//go:embed all:dist
var Dist embed.FS
