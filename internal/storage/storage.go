package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Storage handles persisting received git data
type Storage struct {
	baseDir string
}

// NewStorage creates a new Storage instance
func NewStorage(baseDir string) *Storage {
	return &Storage{baseDir: baseDir}
}

// getRepoDir returns the directory path for a specific repo
func (s *Storage) getRepoDir(repoName string) string {
	return filepath.Join(s.baseDir, repoName)
}

// SavePackData saves raw pack data to disk for a specific repo
func (s *Storage) SavePackData(repoName string, packData []byte, refName string) (string, error) {
	repoDir := s.getRepoDir(repoName)
	
	// Create repo directory if it doesn't exist
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Generate a unique filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	safeRefName := sanitizeRefName(refName)
	filename := fmt.Sprintf("%s-%s.pack", timestamp, safeRefName)
	filepath := filepath.Join(repoDir, filename)

	// Write the pack data
	if err := os.WriteFile(filepath, packData, 0644); err != nil {
		return "", fmt.Errorf("failed to write pack file: %w", err)
	}

	return filepath, nil
}

// SaveObject saves an individual git object to disk for a specific repo
func (s *Storage) SaveObject(repoName string, objType string, sha string, data []byte) error {
	// Create objects directory structure: data/<repo>/objects/<type>/<sha[:2]>/<sha>
	repoDir := s.getRepoDir(repoName)
	objectsDir := filepath.Join(repoDir, "objects", objType, sha[:2])
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return fmt.Errorf("failed to create objects directory: %w", err)
	}

	filepath := filepath.Join(objectsDir, sha)
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write object file: %w", err)
	}

	return nil
}

// SavePushRecord saves metadata about a push for a specific repo
func (s *Storage) SavePushRecord(repoName string, refName, oldSHA, newSHA string, objectCount int) error {
	// Create records directory
	repoDir := s.getRepoDir(repoName)
	recordsDir := filepath.Join(repoDir, "records")
	if err := os.MkdirAll(recordsDir, 0755); err != nil {
		return fmt.Errorf("failed to create records directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	safeRefName := sanitizeRefName(refName)
	filename := fmt.Sprintf("%s-%s.txt", timestamp, safeRefName)
	filepath := filepath.Join(recordsDir, filename)

	content := fmt.Sprintf("Time: %s\nRef: %s\nOld SHA: %s\nNew SHA: %s\nObjects: %d\n",
		time.Now().Format(time.RFC3339),
		refName,
		oldSHA,
		newSHA,
		objectCount,
	)

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write push record: %w", err)
	}

	return nil
}

// SaveHEADS saves the HEADS file for the default branch
// This is stored as data/<repo>/HEADS and contains the ref name in git format: "ref: refs/heads/master"
func (s *Storage) SaveHEADS(repoName string, refName string) error {
	repoDir := s.getRepoDir(repoName)
	
	// Create repo directory if it doesn't exist
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}
	
	// HEADS file contains the ref in git format (e.g., "ref: refs/heads/master")
	headsPath := filepath.Join(repoDir, "HEADS")
	content := fmt.Sprintf("ref: %s\n", refName)
	
	if err := os.WriteFile(headsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write HEADS file: %w", err)
	}
	
	return nil
}

// GetHEADS reads the HEADS file for a repo and returns the default branch ref
func (s *Storage) GetHEADS(repoName string) (string, error) {
	headsPath := filepath.Join(s.getRepoDir(repoName), "HEADS")

	data, err := os.ReadFile(headsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("HEADS file not found for repo %s", repoName)
		}
		return "", fmt.Errorf("failed to read HEADS file: %w", err)
	}

	// Parse "ref: refs/heads/master" format
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: ") {
		return strings.TrimPrefix(content, "ref: "), nil
	}

	return content, nil
}

// GetDefaultBranch returns the default branch name for a repo
func (s *Storage) GetDefaultBranch(repoName string) (string, error) {
	ref, err := s.GetHEADS(repoName)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/"), nil
	}

	return ref, nil
}

// ListRepositories returns all repository names under the base directory
func (s *Storage) ListRepositories() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	repos := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			repos = append(repos, entry.Name())
		}
	}

	return repos, nil
}

// SaveRef saves a reference (branch or tag) under data/<repo>/refs/<type>/<name>
// The file contains the SHA that the reference points to
func (s *Storage) SaveRef(repoName string, refName string, sha string) error {
	// Determine the ref type and name
	var refsDir, refFile string
	
	switch {
	case strings.HasPrefix(refName, "refs/heads/"):
		// Branch reference
		branchName := strings.TrimPrefix(refName, "refs/heads/")
		refsDir = filepath.Join("refs", "heads")
		refFile = branchName
	case strings.HasPrefix(refName, "refs/tags/"):
		// Tag reference
		tagName := strings.TrimPrefix(refName, "refs/tags/")
		refsDir = filepath.Join("refs", "tags")
		refFile = tagName
	default:
		return fmt.Errorf("unsupported ref type: %s", refName)
	}
	
	// Create refs directory structure
	repoDir := s.getRepoDir(repoName)
	fullRefsDir := filepath.Join(repoDir, refsDir)
	if err := os.MkdirAll(fullRefsDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", refsDir, err)
	}
	
	// Write the SHA to the ref file
	refPath := filepath.Join(fullRefsDir, refFile)
	content := fmt.Sprintf("%s\n", sha)
	
	if err := os.WriteFile(refPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write ref file: %w", err)
	}
	
	return nil
}

// GetRef reads a reference and returns the SHA it points to
// refType should be "heads" or "tags"
func (s *Storage) GetRef(repoName string, refType string, name string) (string, error) {
	refPath := filepath.Join(s.getRepoDir(repoName), "refs", refType, name)
	
	data, err := os.ReadFile(refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("ref not found: refs/%s/%s", refType, name)
		}
		return "", fmt.Errorf("failed to read ref file: %w", err)
	}
	
	return strings.TrimSpace(string(data)), nil
}

// ListRefs returns all refs of a given type for a repo
// refType should be "heads" or "tags"
func (s *Storage) ListRefs(repoName string, refType string) ([]string, error) {
	refsDir := filepath.Join(s.getRepoDir(repoName), "refs", refType)
	
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read refs/%s directory: %w", refType, err)
	}
	
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	
	return names, nil
}

// ListBranches returns all branch names for a repo (convenience method)
func (s *Storage) ListBranches(repoName string) ([]string, error) {
	return s.ListRefs(repoName, "heads")
}

// ListTags returns all tag names for a repo (convenience method)
func (s *Storage) ListTags(repoName string) ([]string, error) {
	return s.ListRefs(repoName, "tags")
}

// sanitizeRefName makes a ref name safe for use as a filename
func sanitizeRefName(refName string) string {
	// Replace slashes and other unsafe characters
	result := make([]byte, len(refName))
	for i, b := range []byte(refName) {
		switch b {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result[i] = '_'
		default:
			result[i] = b
		}
	}
	return string(result)
}

// ComputeSHA computes the SHA-1 hash for a git object
func ComputeSHA(objType string, data []byte) string {
	// Git object format: "<type> <size>\0<data>"
	header := fmt.Sprintf("%s %d\x00", objType, len(data))
	fullData := append([]byte(header), data...)

	// Compute SHA-1
	hash := sha1.Sum(fullData)
	return hex.EncodeToString(hash[:])
}

// TreeEntry represents an entry in a git tree object
type TreeEntry struct {
	Mode string
	Name string
	SHA  string
}

// ParseTree parses a git tree object and returns its entries
func ParseTree(data []byte) []TreeEntry {
	var entries []TreeEntry
	i := 0

	for i < len(data) {
		// Format: <mode> <name>\0<20-byte-sha>
		// Find the space after mode
		spaceIdx := -1
		for j := i; j < len(data); j++ {
			if data[j] == ' ' {
				spaceIdx = j
				break
			}
		}
		if spaceIdx == -1 {
			break
		}
		mode := string(data[i:spaceIdx])

		// Find the null byte after name
		nullIdx := -1
		for j := spaceIdx + 1; j < len(data); j++ {
			if data[j] == 0 {
				nullIdx = j
				break
			}
		}
		if nullIdx == -1 {
			break
		}
		name := string(data[spaceIdx+1 : nullIdx])

		// Read 20-byte SHA
		if nullIdx+20 > len(data) {
			break
		}
		sha := hex.EncodeToString(data[nullIdx+1 : nullIdx+21])

		entries = append(entries, TreeEntry{
			Mode: mode,
			Name: name,
			SHA:  sha,
		})

		i = nullIdx + 21
	}

	return entries
}

// GetObject retrieves a git object from storage
func (s *Storage) GetObject(repoName string, objType string, sha string) ([]byte, error) {
	objectPath := filepath.Join(s.getRepoDir(repoName), "objects", objType, sha[:2], sha)

	data, err := os.ReadFile(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s/%s", objType, sha)
		}
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return data, nil
}

// GetFilesFromRepo returns a map of file paths to their contents for a given repo
// It starts from the default branch and traverses the tree
func (s *Storage) GetFilesFromRepo(repoName string) (map[string]string, error) {
	files := make(map[string]string)

	// Get the default branch from HEADS
	headsRef, err := s.GetHEADS(repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get HEADS: %w", err)
	}

	// Parse the ref (e.g., "refs/heads/master")
	var branchName string
	if strings.HasPrefix(headsRef, "refs/heads/") {
		branchName = strings.TrimPrefix(headsRef, "refs/heads/")
	} else {
		return nil, fmt.Errorf("unsupported HEADS ref format: %s", headsRef)
	}

	// Get the commit SHA from refs/heads/<branch>
	commitSHA, err := s.GetRef(repoName, "heads", branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to get ref: %w", err)
	}

	// Get the commit object
	commitData, err := s.GetObject(repoName, "commit", commitSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	// Parse the commit to find the tree SHA
	treeSHA := parseTreeSHAFromCommit(commitData)
	if treeSHA == "" {
		return nil, fmt.Errorf("failed to parse tree SHA from commit")
	}

	// Traverse the tree and collect files
	err = s.traverseTree(repoName, treeSHA, "", files)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse tree: %w", err)
	}

	return files, nil
}

// parseTreeSHAFromCommit extracts the tree SHA from a commit object
func parseTreeSHAFromCommit(data []byte) string {
	// Commit format: "tree <sha>\n..."
	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "tree ") {
			return strings.TrimPrefix(line, "tree ")
		}
	}
	return ""
}

// traverseTree recursively traverses a tree and collects file contents
func (s *Storage) traverseTree(repoName string, treeSHA string, basePath string, files map[string]string) error {
	// Get the tree object
	treeData, err := s.GetObject(repoName, "tree", treeSHA)
	if err != nil {
		return fmt.Errorf("failed to get tree %s: %w", treeSHA, err)
	}

	// Parse tree entries
	entries := ParseTree(treeData)

	for _, entry := range entries {
		fullPath := filepath.Join(basePath, entry.Name)

		switch entry.Mode {
		case "100644", "100755", "0644", "0755":
			// Regular file - get blob content
			blobData, err := s.GetObject(repoName, "blob", entry.SHA)
			if err != nil {
				return fmt.Errorf("failed to get blob %s: %w", entry.SHA, err)
			}
			files[fullPath] = string(blobData)

		case "40000":
			// Directory - recurse into subtree
			err := s.traverseTree(repoName, entry.SHA, fullPath, files)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// TreeNode represents a file or directory in the tree
type TreeNode struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
}

// GetTree returns the tree structure for a branch
func (s *Storage) GetTree(repoName string, branchName string) ([]TreeNode, error) {
	// Get the commit SHA for the branch
	commitSHA, err := s.GetRef(repoName, "heads", branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to get ref: %w", err)
	}

	// Get the commit object
	commitData, err := s.GetObject(repoName, "commit", commitSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	// Parse the commit to find the tree SHA
	treeSHA := parseTreeSHAFromCommit(commitData)
	if treeSHA == "" {
		return nil, fmt.Errorf("failed to parse tree SHA from commit")
	}

	// Get the tree entries
	return s.getTreeNodes(repoName, treeSHA, "")
}

// getTreeNodes returns tree nodes for a given tree SHA
func (s *Storage) getTreeNodes(repoName string, treeSHA string, basePath string) ([]TreeNode, error) {
	// Get the tree object
	treeData, err := s.GetObject(repoName, "tree", treeSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree %s: %w", treeSHA, err)
	}

	// Parse tree entries
	entries := ParseTree(treeData)

	nodes := []TreeNode{}
	for _, entry := range entries {
		node := TreeNode{
			Name: entry.Name,
			Path: filepath.Join(basePath, entry.Name),
		}

		switch entry.Mode {
		case "100644", "100755", "0644", "0755":
			node.Type = "file"
		case "40000":
			node.Type = "dir"
		default:
			// Skip other types (symlinks, submodules, etc.)
			continue
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ResolveTreeEntry resolves a path within a branch to a tree entry
func (s *Storage) ResolveTreeEntry(repoName string, branchName string, path string) (TreeEntry, string, error) {
	commitSHA, err := s.GetRef(repoName, "heads", branchName)
	if err != nil {
		return TreeEntry{}, "", fmt.Errorf("failed to get ref: %w", err)
	}

	commitData, err := s.GetObject(repoName, "commit", commitSHA)
	if err != nil {
		return TreeEntry{}, "", fmt.Errorf("failed to get commit: %w", err)
	}

	treeSHA := parseTreeSHAFromCommit(commitData)
	if treeSHA == "" {
		return TreeEntry{}, "", fmt.Errorf("failed to parse tree SHA from commit")
	}

	cleanPath := strings.Trim(path, "/")
	if cleanPath == "" {
		return TreeEntry{}, treeSHA, nil
	}

	segments := strings.Split(cleanPath, "/")
	currentTreeSHA := treeSHA
	for i, segment := range segments {
		entry, err := s.findTreeEntry(repoName, currentTreeSHA, segment)
		if err != nil {
			return TreeEntry{}, "", err
		}

		if i == len(segments)-1 {
			return entry, "", nil
		}

		if entry.Mode != "40000" {
			return TreeEntry{}, "", fmt.Errorf("path %s is not a directory", segment)
		}

		currentTreeSHA = entry.SHA
	}

	return TreeEntry{}, "", fmt.Errorf("path not found")
}

// findTreeEntry finds an entry by name in a tree
func (s *Storage) findTreeEntry(repoName string, treeSHA string, name string) (TreeEntry, error) {
	treeData, err := s.GetObject(repoName, "tree", treeSHA)
	if err != nil {
		return TreeEntry{}, fmt.Errorf("failed to get tree %s: %w", treeSHA, err)
	}

	entries := ParseTree(treeData)
	for _, entry := range entries {
		if entry.Name == name {
			return entry, nil
		}
	}

	return TreeEntry{}, fmt.Errorf("entry not found: %s", name)
}

// GetTreeNodesAtPath returns tree nodes at a path within a branch
func (s *Storage) GetTreeNodesAtPath(repoName string, branchName string, path string) ([]TreeNode, error) {
	entry, rootTreeSHA, err := s.ResolveTreeEntry(repoName, branchName, path)
	if err != nil {
		return nil, err
	}

	if rootTreeSHA != "" {
		return s.getTreeNodes(repoName, rootTreeSHA, "")
	}

	if entry.Mode != "40000" {
		return nil, fmt.Errorf("path is not a directory")
	}

	return s.getTreeNodes(repoName, entry.SHA, strings.Trim(path, "/"))
}

// GetFileAtPath returns file content at a path within a branch
func (s *Storage) GetFileAtPath(repoName string, branchName string, path string) ([]byte, error) {
	entry, _, err := s.ResolveTreeEntry(repoName, branchName, path)
	if err != nil {
		return nil, err
	}

	if entry.Mode == "40000" {
		return nil, fmt.Errorf("path is a directory")
	}

	return s.GetObject(repoName, "blob", entry.SHA)
}
