// Package assets exposes the North-owned files embedded in the release binary.
package assets

import "embed"

// FS contains agents, hooks, and instruction fragments required by the MVP.
//
//go:embed agents/*.md hooks/*.ts instructions/*.md
var FS embed.FS
