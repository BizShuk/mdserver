// Package server exposes a site over HTTP, rendering markdown per request so
// edits on disk are visible on the next reload.
package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bizshuk/mdserver/svc/render"
	"github.com/bizshuk/mdserver/svc/site"
	"github.com/bizshuk/mdserver/web"
)

const (
	// ASSET_PREFIX namespaces the built-in stylesheet, script and mermaid
	// bundle. It starts with an underscore so it cannot collide with a
	// directory served from the site root, which never has hidden segments.
	ASSET_PREFIX = "/_mdserver/"
	// SYNTAX_CSS_PATH is generated at startup from the chroma themes rather
	// than shipped as a file.
	SYNTAX_CSS_PATH = ASSET_PREFIX + "syntax.css"
	// ASSET_CACHE_CONTROL is safe to set aggressively: the assets are baked
	// into the binary, so they only change when the binary does.
	ASSET_CACHE_CONTROL = "public, max-age=3600"
)

// Server renders a site.Site over HTTP.
type Server struct {
	site      *site.Site
	renderer  *render.Renderer
	template  *template.Template
	syntaxCSS []byte
	assets    http.Handler
	log       *slog.Logger
}

// New wires a Server for the given site.
func New(s *site.Site, logger *slog.Logger) (*Server, error) {
	tmpl, err := template.ParseFS(web.Files, web.PAGE_TEMPLATE)
	if err != nil {
		return nil, fmt.Errorf("parse page template: %w", err)
	}
	syntaxCSS, err := render.SyntaxCSS()
	if err != nil {
		return nil, fmt.Errorf("build syntax css: %w", err)
	}
	assetFS, err := fs.Sub(web.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("open asset fs: %w", err)
	}

	return &Server{
		site:      s,
		renderer:  render.New(),
		template:  tmpl,
		syntaxCSS: syntaxCSS,
		assets:    http.FileServer(http.FS(assetFS)),
		log:       logger,
	}, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(ASSET_PREFIX, s.serveAsset)
	mux.HandleFunc("/", s.serveContent)
	return mux
}

// serveAsset serves the embedded assets, plus the generated syntax stylesheet.
func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == SYNTAX_CSS_PATH {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", ASSET_CACHE_CONTROL)
		_, _ = w.Write(s.syntaxCSS)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, ASSET_PREFIX)
	// mermaid.min.js lives in a subdirectory but is requested flat, so the
	// page template never has to know where it is vendored.
	if name == "mermaid.min.js" {
		name = "vendor/mermaid.min.js"
	}
	if name == "" || name == web.PAGE_TEMPLATE {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", ASSET_CACHE_CONTROL)
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + name
	s.assets.ServeHTTP(w, r2)
}

// serveContent resolves the request path against the site and renders it.
func (s *Server) serveContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.renderError(w, r, http.StatusMethodNotAllowed, "Only GET and HEAD are supported.")
		return
	}

	target, err := s.site.Resolve(r.URL.Path)
	if err != nil {
		s.log.Error("resolve failed",
			slog.String("path", r.URL.Path),
			slog.String("root", s.site.Root),
			slog.String("error", err.Error()))
		s.renderError(w, r, http.StatusBadRequest, "That path could not be resolved.")
		return
	}

	switch target.Kind {
	case site.KIND_REDIRECT:
		http.Redirect(w, r, target.Redirect, http.StatusFound)
	case site.KIND_PAGE:
		s.servePage(w, r, target)
	case site.KIND_DIR:
		s.serveDir(w, r, target)
	case site.KIND_STATIC:
		s.serveStatic(w, r, target)
	default:
		s.renderError(w, r, http.StatusNotFound, "No page or directory matches "+r.URL.Path+".")
	}
}

// serveStatic hands non-markdown files to the standard file server, which
// handles range requests, content types and conditional GETs.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request, target site.Target) {
	file, err := os.Open(target.FSPath)
	if err != nil {
		s.renderError(w, r, http.StatusNotFound, "That file could not be read.")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		s.renderError(w, r, http.StatusNotFound, "That file could not be read.")
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// modTimeText formats a modification time for the page footer.
func modTimeText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}
