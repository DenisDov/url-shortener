// Package web embeds the static frontend so the binary stays self-contained --
// no asset directory to ship next to it, no build step, no separate server.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

// Static returns the asset tree rooted at the directory holding index.html.
func Static() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		// Only reachable if the embed directive above stops matching, which
		// the compiler would already have caught.
		panic(err)
	}
	return sub
}
