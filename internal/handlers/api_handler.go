package handlers

import (
	"net/http"
	"strings"

	"gitprovider/internal/storage"

	"github.com/gin-gonic/gin"
)

// RepoInfo represents repository metadata returned by the API
type RepoInfo struct {
	Name          string   `json:"name"`
	DefaultBranch string   `json:"defaultBranch"`
	Branches      []string `json:"branches"`
	URL           string   `json:"url"`
}

// APIHandler handles REST API requests
type APIHandler struct {
	storage *storage.Storage
}

// NewAPIHandler creates a new APIHandler
func NewAPIHandler() *APIHandler {
	return &APIHandler{
		storage: storage.NewStorage("data"),
	}
}

// OpenAPIResponse represents a minimal OpenAPI response
type OpenAPIResponse struct {
	OpenAPI string                 `json:"openapi"`
	Info    OpenAPIInfo            `json:"info"`
	Paths   map[string]interface{} `json:"paths"`
}

// OpenAPIInfo represents the OpenAPI info section
type OpenAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// GetOpenAPI handles GET /
func (h *APIHandler) GetOpenAPI(c *gin.Context) {
	response := OpenAPIResponse{
		OpenAPI: "3.0.0",
		Info: OpenAPIInfo{
			Title:   "Git Provider API",
			Version: "0.1.0",
		},
		Paths: map[string]interface{}{
			"/": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "OpenAPI root",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OpenAPI document",
						},
					},
				},
			},
			"/repos": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "List repositories",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Repository list",
						},
					},
				},
			},
			"/repos/{repo}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Get repository details",
					"parameters": []map[string]interface{}{
						{
							"name":     "repo",
							"in":       "path",
							"required": true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Repository details",
						},
						"404": map[string]interface{}{
							"description": "Repository not found",
						},
					},
				},
			},
			"/repos/{repo}/tree/{branch}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Get repository tree",
					"parameters": []map[string]interface{}{
						{
							"name":     "repo",
							"in":       "path",
							"required": true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":     "branch",
							"in":       "path",
							"required": true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Repository tree",
						},
						"404": map[string]interface{}{
							"description": "Repository or branch not found",
						},
					},
				},
			},
			"/repos/{repo}/tree/{branch}/{path}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Get repository tree path",
					"parameters": []map[string]interface{}{
						{
							"name":     "repo",
							"in":       "path",
							"required": true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":     "branch",
							"in":       "path",
							"required": true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":     "path",
							"in":       "path",
							"required": true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Directory listing or file content",
						},
						"404": map[string]interface{}{
							"description": "Repository, branch, or path not found",
						},
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, response)
}

// ListRepos handles GET /repos
func (h *APIHandler) ListRepos(c *gin.Context) {
	repos, err := h.storage.ListRepositories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	baseURL := getBaseURL(c)

	result := []RepoInfo{}
	for _, repo := range repos {
		defaultBranch, err := h.storage.GetDefaultBranch(repo)
		if err != nil {
			defaultBranch = ""
		}

		branches, err := h.storage.ListBranches(repo)
		if err != nil {
			branches = []string{}
		}

		result = append(result, RepoInfo{
			Name:          repo,
			DefaultBranch: defaultBranch,
			Branches:      branches,
			URL:           baseURL + "/" + repo + ".git",
		})
	}

	c.JSON(http.StatusOK, result)
}

// GetRepo handles GET /repos/:repo
func (h *APIHandler) GetRepo(c *gin.Context) {
	repoName := c.Param("repo")

	// Check if repo exists
	repos, err := h.storage.ListRepositories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	found := false
	for _, r := range repos {
		if r == repoName {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	defaultBranch, err := h.storage.GetDefaultBranch(repoName)
	if err != nil {
		defaultBranch = ""
	}

	branches, err := h.storage.ListBranches(repoName)
	if err != nil {
		branches = []string{}
	}

	baseURL := getBaseURL(c)

	result := RepoInfo{
		Name:          repoName,
		DefaultBranch: defaultBranch,
		Branches:      branches,
		URL:           baseURL + "/" + repoName + ".git",
	}

	c.JSON(http.StatusOK, result)
}

// GetTree handles GET /repos/:repo/tree/:branch
func (h *APIHandler) GetTree(c *gin.Context) {
	repoName := c.Param("repo")
	branch := c.Param("branch")

	// Check if repo exists
	repos, err := h.storage.ListRepositories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	found := false
	for _, r := range repos {
		if r == repoName {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	tree, err := h.storage.GetTree(repoName, branch)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tree)
}

// GetTreePath handles GET /repos/:repo/tree/:branch/*path
func (h *APIHandler) GetTreePath(c *gin.Context) {
	repoName := c.Param("repo")
	branch := c.Param("branch")
	path := strings.TrimPrefix(c.Param("path"), "/")

	// Check if repo exists
	repos, err := h.storage.ListRepositories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	found := false
	for _, r := range repos {
		if r == repoName {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}

	// Try to list directory nodes at path
	if treeNodes, err := h.storage.GetTreeNodesAtPath(repoName, branch, path); err == nil {
		c.JSON(http.StatusOK, treeNodes)
		return
	}

	// If not a directory, try to return file content
	fileData, err := h.storage.GetFileAtPath(repoName, branch, path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", fileData)
}

// getBaseURL constructs the base URL from the request
func getBaseURL(c *gin.Context) string {
	baseURL := strings.TrimSuffix(c.Request.Host, "/")
	if c.Request.TLS == nil {
		baseURL = "http://" + baseURL
	} else {
		baseURL = "https://" + baseURL
	}
	return baseURL
}

// RegisterRoutes registers API routes
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/", h.GetOpenAPI)
	r.GET("/repos", h.ListRepos)
	r.GET("/repos/:repo", h.GetRepo)
	r.GET("/repos/:repo/tree/:branch", h.GetTree)
	r.GET("/repos/:repo/tree/:branch/*path", h.GetTreePath)
}