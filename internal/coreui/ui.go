// Package coreui embeds only the independently built public interface.
package coreui

import (
	"embed"
	"github.com/axottle/axottle/internal/staticui"
	"io/fs"
	"net/http"
)

//go:embed all:static
var files embed.FS

func Handler() http.Handler {
	sub, _ := fs.Sub(files, "static")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { staticui.Serve(w, r, sub) })
}
