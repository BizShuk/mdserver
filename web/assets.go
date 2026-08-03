// Package web holds the static assets and page template compiled into the
// binary, so mdserver needs nothing on disk but the markdown it serves.
package web

import "embed"

// Files contains every asset served under the internal asset prefix, plus the
// page template.
//
//go:embed page.html style.css app.js vendor/mermaid.min.js
var Files embed.FS

// PAGE_TEMPLATE is the name of the single HTML template.
const PAGE_TEMPLATE = "page.html"
