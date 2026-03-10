package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"gitprovider/internal/storage"

	"github.com/gin-gonic/gin"
)

// RepoHandler handles repository file access requests
type RepoHandler struct {
	storage *storage.Storage
}

// NewRepoHandler creates a new RepoHandler
func NewRepoHandler() *RepoHandler {
	return &RepoHandler{
		storage: storage.NewStorage("data"),
	}
}

// NewRepoHandlerWithStorage creates a new RepoHandler with a custom storage directory
func NewRepoHandlerWithStorage(dataDir string) *RepoHandler {
	return &RepoHandler{
		storage: storage.NewStorage(dataDir),
	}
}

// GetFile handles GET /repos/:repo/*filepath
// Returns the content of a file from a repository
func (h *RepoHandler) GetFile(c *gin.Context) {
	// Parse path directly since we're using NoRoute
	path := c.Request.URL.Path
	// Expected format: /repos/<repo>/<filepath>
	path = strings.TrimPrefix(path, "/repos/")
	parts := strings.SplitN(path, "/", 2)

	repoName := parts[0]
	var filepath string
	if len(parts) > 1 {
		filepath = "/" + parts[1]
	}

	fmt.Printf("[INFO] GetFile request: repo=%s, filepath=%s\n", repoName, filepath)

	// TODO: Implement actual file lookup based on repoName and filepath
	// For now, hardcoded to return hello.txt content from test1 repo

	// Hardcoded response for testing
	if repoName == "test1" {
		// Return hardcoded hello.txt content
		c.String(http.StatusOK, "Hello World\n")
		return
	}

	c.String(http.StatusNotFound, "Repository not found: %s", repoName)
}

// RegisterRoutes registers the repo routes on the gin engine
func (h *RepoHandler) RegisterRoutes(r *gin.Engine) {
	// No longer used - routing is handled via main.go NoRoute
}