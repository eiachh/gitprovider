package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gitprovider/internal/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	port := getPort()

	r := gin.New()
	r.Use(gin.Logger())

	// Create handlers
	apiHandler := handlers.NewAPIHandler()
	gitHandler := handlers.NewGitHandler()

	// Register API routes
	r.GET("/", apiHandler.GetOpenAPI)
	r.GET("/repos", apiHandler.ListRepos)
	r.GET("/repos/:repo", apiHandler.GetRepo)
	r.GET("/repos/:repo/tree/:branch", apiHandler.GetTree)
	r.GET("/repos/:repo/tree/:branch/*path", apiHandler.GetTreePath)

	// Register a catch-all route for git protocol paths
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Route to git handler for git protocol paths
		if strings.HasSuffix(path, "/info/refs") || strings.HasSuffix(path, "/git-receive-pack") {
			gitHandler.HandleRequest(c)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	// Start server with dynamic or configured port
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	// Print the actual port being used
	actualPort := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("Server starting on port %d\n", actualPort)

	if err := r.RunListener(listener); err != nil {
		panic(err)
	}
}

func getPort() int {
	portStr := os.Getenv("GIT_SERVER_PORT")
	if portStr == "" {
		return 0 // Dynamic port
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}
