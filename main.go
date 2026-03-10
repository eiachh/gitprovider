package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"gitprovider/internal/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	port := getPort()

	r := gin.New()

	// Create git handler and register routes
	gitHandler := handlers.NewGitHandler()
	gitHandler.RegisterRoutes(r)

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
