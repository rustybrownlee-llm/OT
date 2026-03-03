package web

// handlers_editor.go implements the GET /design/edit page handler and the
// file tree data assembly used to populate the editor sidebar.

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rustybrownlee/ot-simulator/admin/internal/schema"
)

// FileTreeNode represents one entry in the design directory file tree.
// Directories have IsDir=true and a Children slice; files have IsDir=false.
type FileTreeNode struct {
	Name     string         // display name (filename or directory basename)
	Path     string         // relative path from design directory (used in hrefs)
	IsDir    bool           // true for directories
	Children []FileTreeNode // populated for directories only
}

// editorPageData is the template data for the YAML editor (/design/edit) page.
type editorPageData struct {
	Title      string
	ActivePage string
	FilePath   string         // relative path of the open file (from query param)
	SchemaType string         // OT-facing label for the schema type badge
	FileTree   []FileTreeNode // top-level tree nodes (devices/, networks/, environments/)
}

// schemaTypeLabel maps a schema.SchemaType to an OT-facing display label.
func schemaTypeLabel(st schema.SchemaType) string {
	switch st {
	case schema.SchemaTypeDeviceAtom:
		return "Device Profile"
	case schema.SchemaTypeNetworkAtom:
		return "Network Segment"
	case schema.SchemaTypeEnvironment:
		return "Environment Definition"
	case schema.SchemaTypeProcess:
		return "Process Definition"
	default:
		return ""
	}
}

// editorHandler handles GET /design/edit.
// Renders the three-panel YAML editor page. The ?file= query parameter
// specifies which design file to open; if absent the page renders without
// an open file and the editor is empty.
func (s *Server) editorHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildEditorData(r)
	s.render(w, "editor.html", data)
}

// buildEditorData assembles editorPageData for the editor page.
func (s *Server) buildEditorData(r *http.Request) editorPageData {
	filePath := r.URL.Query().Get("file")

	data := editorPageData{
		Title:      "YAML Editor",
		ActivePage: "design",
		FilePath:   filePath,
		FileTree:   buildFileTree(s.globals.DesignDir),
	}

	if filePath != "" {
		absPath := filepath.Join(s.globals.DesignDir, filePath)
		st, err := schema.InferSchemaType(absPath, s.globals.DesignDir)
		if err == nil {
			data.SchemaType = schemaTypeLabel(st)
		}
	}

	return data
}

// buildFileTree scans the design directory and returns a slice of top-level
// FileTreeNode entries for devices/, networks/, and environments/.
// Only .yaml files are included. Entries are sorted alphabetically.
func buildFileTree(designDir string) []FileTreeNode {
	if designDir == "" {
		return nil
	}

	topDirs := []string{"devices", "networks", "environments"}
	var nodes []FileTreeNode

	for _, dir := range topDirs {
		node := scanDirectory(designDir, dir)
		if node != nil {
			nodes = append(nodes, *node)
		}
	}

	return nodes
}

// scanDirectory builds a FileTreeNode for the given subdirectory of designDir.
// Returns nil if the directory does not exist or cannot be read.
func scanDirectory(designDir, subDir string) *FileTreeNode {
	dirPath := filepath.Join(designDir, subDir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	node := &FileTreeNode{
		Name:  subDir,
		Path:  subDir,
		IsDir: true,
	}

	for _, e := range entries {
		if e.IsDir() {
			child := scanSubDirectory(designDir, subDir, e.Name())
			if child != nil {
				node.Children = append(node.Children, *child)
			}
		} else if strings.HasSuffix(e.Name(), ".yaml") {
			node.Children = append(node.Children, FileTreeNode{
				Name: e.Name(),
				Path: subDir + "/" + e.Name(),
			})
		}
	}

	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Name < node.Children[j].Name
	})

	return node
}

// scanSubDirectory builds a FileTreeNode for a second-level directory
// (e.g., environments/brownfield-wastewater/).
func scanSubDirectory(designDir, subDir, name string) *FileTreeNode {
	dirPath := filepath.Join(designDir, subDir, name)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	node := &FileTreeNode{
		Name:  name,
		Path:  subDir + "/" + name,
		IsDir: true,
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			node.Children = append(node.Children, FileTreeNode{
				Name: e.Name(),
				Path: subDir + "/" + name + "/" + e.Name(),
			})
		}
	}

	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Name < node.Children[j].Name
	})

	return node
}
