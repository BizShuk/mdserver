package server

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/bizshuk/mdserver/svc/render"
	"github.com/bizshuk/mdserver/svc/site"
	"github.com/bizshuk/mdserver/web"
)

// pageData is the template contract for every response.
type pageData struct {
	SiteTitle    string
	Title        string
	Description  string
	DirName      string
	Breadcrumbs  []crumb
	SidebarTitle string
	Siblings     []navItem
	ParentURL    string
	Content      template.HTML
	Entries      []site.Entry
	EntryCount   int
	TOC          []render.Heading
	HasMermaid   bool
	IsDir        bool
	SourcePath   string
	ModTime      string
	AssetPrefix  string
	Error        *errorInfo
}

type crumb struct {
	Name string
	URL  string
}

type navItem struct {
	Label     string
	URL       string
	IsDir     bool
	PageCount int
	Current   bool
}

type errorInfo struct {
	Status  int
	Message string
}

// servePage renders a single markdown file.
func (s *Server) servePage(w http.ResponseWriter, r *http.Request, target site.Target) {
	source, err := os.ReadFile(target.FSPath)
	if err != nil {
		s.log.Error("read page failed",
			slog.String("path", r.URL.Path),
			slog.String("fs_path", target.FSPath),
			slog.String("error", err.Error()))
		s.renderError(w, r, http.StatusNotFound, "That page could not be read.")
		return
	}

	doc, err := s.renderer.Render(source)
	if err != nil {
		s.log.Error("render page failed",
			slog.String("path", r.URL.Path),
			slog.String("fs_path", target.FSPath),
			slog.String("error", err.Error()))
		s.renderError(w, r, http.StatusInternalServerError, "That page could not be rendered.")
		return
	}

	data := s.baseData(target.URLPath)
	data.Title = fallback(doc.Title, strings.TrimSuffix(path.Base(target.RelPath), site.MD_EXTENSION))
	data.Description = doc.Description
	data.Content = doc.HTML
	data.TOC = render.TOC(doc.Headings)
	data.HasMermaid = doc.HasMermaid
	data.SourcePath = target.RelPath
	if info, err := os.Stat(target.FSPath); err == nil {
		data.ModTime = modTimeText(info.ModTime())
	}
	s.populateNav(&data, path.Dir(target.URLPath), path.Base(target.URLPath))

	s.writePage(w, r, http.StatusOK, data)
}

// serveDir renders a directory: its index page, if any, followed by a listing
// of everything the directory contains.
func (s *Server) serveDir(w http.ResponseWriter, r *http.Request, target site.Target) {
	data := s.baseData(target.URLPath)
	data.IsDir = true
	data.DirName = fallback(path.Base(strings.TrimSuffix(target.URLPath, "/")), s.site.Title)
	data.SourcePath = fallback(target.RelPath, ".")

	if target.IndexPath != "" {
		source, err := os.ReadFile(target.IndexPath)
		if err == nil {
			doc, err := s.renderer.Render(source)
			if err != nil {
				s.log.Error("render index failed",
					slog.String("path", r.URL.Path),
					slog.String("fs_path", target.IndexPath),
					slog.String("error", err.Error()))
			} else {
				data.Title = doc.Title
				data.Description = doc.Description
				data.Content = doc.HTML
				data.TOC = render.TOC(doc.Headings)
				data.HasMermaid = doc.HasMermaid
			}
		}
		if info, err := os.Stat(target.IndexPath); err == nil {
			data.ModTime = modTimeText(info.ModTime())
		}
	}

	entries, err := s.site.Listing(target)
	if err != nil {
		s.log.Error("listing failed",
			slog.String("path", r.URL.Path),
			slog.String("fs_path", target.FSPath),
			slog.String("error", err.Error()))
		s.renderError(w, r, http.StatusInternalServerError, "That directory could not be listed.")
		return
	}
	data.Entries = entries
	data.EntryCount = len(entries)
	// A directory lists its own contents in the main column, so the sidebar
	// shows the parent's contents instead of repeating them.
	s.populateNav(&data, path.Dir(strings.TrimSuffix(target.URLPath, "/")), path.Base(strings.TrimSuffix(target.URLPath, "/")))

	s.writePage(w, r, http.StatusOK, data)
}

// renderError renders an error using the same layout as content pages.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	data := s.baseData(r.URL.Path)
	data.Title = http.StatusText(status)
	data.Error = &errorInfo{Status: status, Message: message}
	s.writePage(w, r, status, data)
}

// writePage buffers the template output so a template failure cannot emit a
// half-written page under a 200 status.
func (s *Server) writePage(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	var buf bytes.Buffer
	if err := s.template.ExecuteTemplate(&buf, web.PAGE_TEMPLATE, data); err != nil {
		s.log.Error("execute template failed",
			slog.String("path", r.URL.Path),
			slog.String("title", data.Title),
			slog.String("error", err.Error()))
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Markdown is re-read per request; caching would defeat that.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) baseData(urlPath string) pageData {
	return pageData{
		SiteTitle:   s.site.Title,
		Breadcrumbs: breadcrumbs(urlPath),
		AssetPrefix: ASSET_PREFIX,
	}
}

// populateNav fills the sidebar with the contents of dirURL, marking
// currentName as the active entry.
func (s *Server) populateNav(data *pageData, dirURL, currentName string) {
	if dirURL == "." || dirURL == "" {
		dirURL = "/"
	}
	if !strings.HasSuffix(dirURL, "/") {
		dirURL += "/"
	}

	target, err := s.site.Resolve(dirURL)
	if err != nil || target.Kind != site.KIND_DIR {
		return
	}
	entries, err := s.site.Listing(target)
	if err != nil {
		return
	}

	data.SidebarTitle = fallback(path.Base(strings.TrimSuffix(dirURL, "/")), s.site.Title)
	if dirURL != "/" {
		data.ParentURL = parentURL(dirURL)
	}

	decodedCurrent, err := url.PathUnescape(currentName)
	if err != nil {
		decodedCurrent = currentName
	}

	data.Siblings = make([]navItem, 0, len(entries))
	for _, entry := range entries {
		label := entry.Name
		if !entry.IsDir && entry.Title != "" {
			label = entry.Title
		}
		data.Siblings = append(data.Siblings, navItem{
			Label:     label,
			URL:       dirURL + entry.URL,
			IsDir:     entry.IsDir,
			PageCount: entry.PageCount,
			Current:   entry.Name == decodedCurrent,
		})
	}
}

// breadcrumbs turns a URL path into a trail, with the site root first and the
// current segment unlinked.
func breadcrumbs(urlPath string) []crumb {
	trimmed := strings.Trim(urlPath, "/")
	if trimmed == "" {
		return []crumb{{Name: "home"}}
	}

	segments := strings.Split(trimmed, "/")
	trail := make([]crumb, 0, len(segments)+1)
	trail = append(trail, crumb{Name: "home", URL: "/"})

	prefix := ""
	for i, segment := range segments {
		prefix += "/" + segment
		name, err := url.PathUnescape(segment)
		if err != nil {
			name = segment
		}
		if i == len(segments)-1 {
			trail = append(trail, crumb{Name: name})
			continue
		}
		trail = append(trail, crumb{Name: name, URL: prefix + "/"})
	}
	return trail
}

// parentURL returns the directory URL one level above dirURL.
func parentURL(dirURL string) string {
	parent := path.Dir(strings.TrimSuffix(dirURL, "/"))
	if parent == "." || parent == "/" {
		return "/"
	}
	return parent + "/"
}

func fallback(value, alternative string) string {
	if strings.TrimSpace(value) == "" {
		return alternative
	}
	return value
}
