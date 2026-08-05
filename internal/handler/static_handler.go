package handler

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// StaticHandler serves the embedded frontend from the same origin as the API,
// which is the reason none of this needs CORS.
type StaticHandler struct {
	fsys   fs.FS
	assets http.Handler
}

func NewStaticHandler(fsys fs.FS) *StaticHandler {
	return &StaticHandler{
		fsys:   fsys,
		assets: http.StripPrefix("/static", http.FileServerFS(fsys)),
	}
}

func (h *StaticHandler) Routes(r chi.Router) {
	r.Get("/", h.Index)
	r.Handle("/static/*", h.assets)
}

// Index serves the app shell. It is registered on "/" alone: chi gives the
// literal "/static" segment priority over "/{code}", and "/" never matches
// "/{code}", so adding the UI cannot shadow a short code -- except the code
// "static", which is now unreachable out of 62^7 possibilities.
func (h *StaticHandler) Index(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, h.fsys, "index.html")
}
