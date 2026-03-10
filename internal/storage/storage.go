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
