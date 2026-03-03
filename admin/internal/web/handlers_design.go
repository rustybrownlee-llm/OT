package web

import (
	"net/http"
)

// designPageData is the template data for the Design Library (/design) page.
type designPageData struct {
	Title      string
	ActivePage string
	Cache      *DesignCache
}

// designHandler renders the full Design Library (/design) page.
func (s *Server) designHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildDesignData()
	s.render(w, "design.html", data)
}

// buildDesignData assembles the data for the Design Library page.
// The design cache is pre-loaded at startup; this function simply wraps it.
func (s *Server) buildDesignData() designPageData {
	return designPageData{
		Title:      "Design Library",
		ActivePage: "design",
		Cache:      s.design,
	}
}
