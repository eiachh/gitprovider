package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"gitprovider/internal/storage"
	"gitprovider/pkg/pack"
	"gitprovider/pkg/protocol"

	"github.com/gin-gonic/gin"
)

// GitHandler handles git protocol requests
type GitHandler struct {
	storage *storage.Storage
}

// NewGitHandler creates a new GitHandler
func NewGitHandler() *GitHandler {
	return &GitHandler{
		storage: storage.NewStorage("data"),
	}
}

// NewGitHandlerWithStorage creates a new GitHandler with a custom storage directory
func NewGitHandlerWithStorage(dataDir string) *GitHandler {
	return &GitHandler{
		storage: storage.NewStorage(dataDir),
	}
}

// extractRepoNameFromParam extracts the repository name from the gin path parameter
// The param will be like "repo.git" or "myproject.git"
func extractRepoNameFromParam(param string) string {
	// Remove .git suffix
	if strings.HasSuffix(param, ".git") {
		return strings.TrimSuffix(param, ".git")
	}
	return param
}

// extractRepoName extracts the repository name from the URL path
// URL pattern: /<repo>.git/info/refs or /<repo>.git/git-receive-pack
func extractRepoName(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	
	// Extract repo name (everything before .git)
	idx := strings.Index(path, ".git")
	if idx > 0 {
		return path[:idx]
	}
	
	// Fallback: use "default" if we can't parse
	return "default"
}

// InfoRefs handles GET /<repo>.git/info/refs
// This endpoint is called by git client to discover refs and capabilities
func (h *GitHandler) InfoRefs(c *gin.Context) {
	service := c.Query("service")
	repoName := extractRepoName(c.Request.URL.Path)
	
	fmt.Printf("[INFO] Incoming request: GET %s?service=%s\n", c.Request.URL.Path, service)
	fmt.Printf("[INFO] Repository: %s\n", repoName)
	fmt.Printf("[INFO] Request headers: %v\n", c.Request.Header)

	if service != "git-receive-pack" {
		fmt.Printf("[WARN] Rejected: unsupported service '%s'\n", service)
		c.String(http.StatusForbidden, "Only git-receive-pack is supported")
		return
	}

	// Set content type for git protocol
	c.Header("Content-Type", "application/x-git-receive-pack-advertisement")
	c.Header("Cache-Control", "no-cache")

	// Create and encode the response
	response := protocol.NewInfoRefsResponse()
	responseData := response.Encode()

	fmt.Printf("[INFO] Outgoing response: %d bytes\n", len(responseData))
	fmt.Printf("[DEBUG] Response content:\n%s\n", formatPktLines(responseData))

	c.Writer.Write(responseData)
}

// ReceivePack handles POST /<repo>.git/git-receive-pack
// This endpoint receives the git push data
func (h *GitHandler) ReceivePack(c *gin.Context) {
	repoName := extractRepoName(c.Request.URL.Path)
	
	fmt.Printf("[INFO] Incoming request: POST %s\n", c.Request.URL.Path)
	fmt.Printf("[INFO] Repository: %s\n", repoName)
	fmt.Printf("[INFO] Request headers: %v\n", c.Request.Header)
	fmt.Printf("[INFO] Content-Length: %d\n", c.Request.ContentLength)

	c.Header("Content-Type", "application/x-git-receive-pack-result")

	// Read the entire request body
	body, err := c.GetRawData()
	if err != nil {
		fmt.Printf("[ERROR] Failed to read request body: %v\n", err)
		c.String(http.StatusInternalServerError, "Failed to read request body")
		return
	}

	fmt.Printf("[INFO] Received body: %d bytes\n", len(body))

	// Parse the receive-pack request
	request, err := protocol.ParseReceivePackRequest(body)
	if err != nil {
		fmt.Printf("[ERROR] Failed to parse request: %v\n", err)
		c.String(http.StatusBadRequest, "Failed to parse request: %v", err)
		return
	}

	// Log parsed commands
	fmt.Printf("[INFO] Parsed %d command(s):\n", len(request.Commands))
	for i, cmd := range request.Commands {
		oldShaDisplay := cmd.OldSHA
		newShaDisplay := cmd.NewSHA
		if len(oldShaDisplay) > 8 {
			oldShaDisplay = oldShaDisplay[:8]
		}
		if len(newShaDisplay) > 8 {
			newShaDisplay = newShaDisplay[:8]
		}
		fmt.Printf("[INFO]   Command %d: %s -> %s %s\n", i+1, oldShaDisplay, newShaDisplay, cmd.RefName)
		if len(cmd.Capabilities) > 0 {
			fmt.Printf("[INFO]     Capabilities: %v\n", cmd.Capabilities)
		}
	}

	// Parse and save pack data
	var packFile *pack.PackFile
	if len(request.PackData) > 0 {
		fmt.Printf("[INFO] Parsing pack data: %d bytes\n", len(request.PackData))

		packFile, err = pack.ParsePackFile(request.PackData)
		if err != nil {
			fmt.Printf("[ERROR] Failed to parse pack file: %v\n", err)
			// Still continue to accept the push, but log the error
		} else {
			fmt.Printf("[INFO] Pack file parsed successfully:\n")
			fmt.Printf("[INFO]   Version: %d\n", packFile.Header.Version)
			fmt.Printf("[INFO]   Object count: %d\n", packFile.Header.ObjectCount)

			// Log each object
			for i, obj := range packFile.Objects {
				fmt.Printf("[INFO]   Object %d: type=%s size=%d decompressed=%d bytes\n",
					i+1, obj.Type, obj.Size, len(obj.Data))
				if obj.BaseSHA != "" {
					fmt.Printf("[INFO]     Base SHA: %s\n", obj.BaseSHA[:8])
				}
				if obj.BaseOffset > 0 {
					fmt.Printf("[INFO]     Base offset: %d\n", obj.BaseOffset)
				}

				// Compute and log SHA for non-delta objects
				if obj.Type != pack.ObjectOfsDelta && obj.Type != pack.ObjectRefDelta {
					sha := storage.ComputeSHA(obj.Type.String(), obj.Data)
					fmt.Printf("[INFO]     Computed SHA: %s\n", sha)

					// Save the object
					if err := h.storage.SaveObject(repoName, obj.Type.String(), sha, obj.Data); err != nil {
						fmt.Printf("[WARN] Failed to save object %s: %v\n", sha[:8], err)
					} else {
						fmt.Printf("[INFO]     Saved object to data/%s/objects/%s/%s/%s\n",
							repoName, obj.Type.String(), sha[:2], sha)
					}
				}
			}

			// Save the raw pack file, HEADS, and refs for each command
			for _, cmd := range request.Commands {
				packPath, err := h.storage.SavePackData(repoName, request.PackData, cmd.RefName)
				if err != nil {
					fmt.Printf("[WARN] Failed to save pack data: %v\n", err)
				} else {
					fmt.Printf("[INFO] Saved pack file to: %s\n", packPath)
				}

				// Save push record
				if err := h.storage.SavePushRecord(repoName, cmd.RefName, cmd.OldSHA, cmd.NewSHA, len(packFile.Objects)); err != nil {
					fmt.Printf("[WARN] Failed to save push record: %v\n", err)
				}

				// Save HEADS file for the default branch (first time we see this ref)
				// This stores the default branch for the repo
				if err := h.storage.SaveHEADS(repoName, cmd.RefName); err != nil {
					fmt.Printf("[WARN] Failed to save HEADS file: %v\n", err)
				} else {
					fmt.Printf("[INFO] Saved HEADS: %s -> %s\n", repoName, cmd.RefName)
				}

				// Save the branch reference under refs/heads/<name>
				if err := h.storage.SaveRef(repoName, cmd.RefName, cmd.NewSHA); err != nil {
					fmt.Printf("[WARN] Failed to save ref %s: %v\n", cmd.RefName, err)
				} else {
					fmt.Printf("[INFO] Saved ref: %s -> %s\n", cmd.RefName, cmd.NewSHA)
				}
			}
		}
	}

	// Build the response
	var response []byte

	// Respond with unpack ok
	response = append(response, protocol.EncodePktLine("unpack ok\n")...)

	// Acknowledge each ref update
	for _, cmd := range request.Commands {
		response = append(response, protocol.EncodePktLine("ok "+cmd.RefName+"\n")...)
	}

	// End with flush packet
	response = append(response, protocol.EncodeFlush()...)

	fmt.Printf("[INFO] Outgoing response: %d bytes\n", len(response))
	fmt.Printf("[DEBUG] Response content:\n%s\n", formatPktLines(response))

	c.Writer.Write(response)
}

// formatPktLines formats pkt-line data for readable logging
func formatPktLines(data []byte) string {
	result := ""
	offset := 0
	for offset < len(data) {
		if offset+4 > len(data) {
			result += fmt.Sprintf("  (truncated data at offset %d)\n", offset)
			break
		}

		lengthHex := string(data[offset : offset+4])
		if lengthHex == "0000" {
			result += "  [FLUSH]\n"
			offset += 4
			continue
		}

		var length int
		_, err := fmt.Sscanf(lengthHex, "%04x", &length)
		if err != nil {
			result += fmt.Sprintf("  (invalid length '%s' at offset %d)\n", lengthHex, offset)
			break
		}

		if length == 0 {
			result += "  [FLUSH]\n"
			offset += 4
			continue
		}

		end := offset + length
		if end > len(data) {
			result += fmt.Sprintf("  (truncated line at offset %d, expected %d bytes)\n", offset, length)
			break
		}

		content := data[offset+4 : end]
		result += fmt.Sprintf("  %s: %q\n", lengthHex, string(content))
		offset = end
	}
	return result
}

// RegisterRoutes registers the git protocol routes on the gin engine
// Supports dynamic repo names: /<repo>.git/info/refs and /<repo>.git/git-receive-pack
func (h *GitHandler) RegisterRoutes(r *gin.Engine) {
	// Register catch-all handlers for both GET and POST
	r.GET("/*path", func(c *gin.Context) {
		h.handleGitRequest(c)
	})
	r.POST("/*path", func(c *gin.Context) {
		h.handleGitRequest(c)
	})
}

// handleGitRequest routes git requests to the appropriate handler
func (h *GitHandler) handleGitRequest(c *gin.Context) {
	path := c.Request.URL.Path
	
	// Check if it's a git request
	if strings.HasSuffix(path, "/info/refs") {
		h.InfoRefs(c)
		return
	}
	
	if strings.HasSuffix(path, "/git-receive-pack") {
		h.ReceivePack(c)
		return
	}
	
	// Not a git request
	c.String(http.StatusNotFound, "Not found")
}
