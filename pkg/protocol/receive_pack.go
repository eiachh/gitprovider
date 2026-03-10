package protocol

import (
	"bytes"
	"fmt"
	"strings"
)

// ReceivePackCommand represents a single ref update command in git-receive-pack
type ReceivePackCommand struct {
	OldSHA       string
	NewSHA       string
	RefName      string
	Capabilities []string
}

// ReceivePackRequest represents the parsed git-receive-pack request
// It contains the commands and capabilities before the PACK data
type ReceivePackRequest struct {
	Commands    []*ReceivePackCommand
	Capabilities []string
	// PackData contains the raw pack data (not parsed yet)
	PackData []byte
}

// ParseReceivePackRequest parses the body of a git-receive-pack request
// It parses the commands section and returns the parsed request
// The PACK data section is left unparsed
func ParseReceivePackRequest(body []byte) (*ReceivePackRequest, error) {
	request := &ReceivePackRequest{
		Commands: []*ReceivePackCommand{},
	}

	// Read all pkt-lines from the body
	lines, err := ReadAllPktLines(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read pkt-lines: %w", err)
	}

	// Find where the pack data starts
	// The format is: commands (pkt-lines) + flush + pack data
	packStart := findPackStart(body)

	// If we have pack data, extract it starting from "PACK" signature
	if packStart >= 0 && packStart < len(body) {
		request.PackData = body[packStart:]
	}

	// Parse each command line
	for _, line := range lines {
		cmd, err := parseCommandLine(string(line.Data))
		if err != nil {
			continue // Skip invalid lines
		}
		if cmd != nil {
			request.Commands = append(request.Commands, cmd)
		}
	}

	return request, nil
}

// parseCommandLine parses a single command line
// Format: <old-sha> <new-sha> <ref-name>\0<capabilities>
func parseCommandLine(data string) (*ReceivePackCommand, error) {
	// Split by null byte to separate capabilities
	parts := strings.SplitN(data, "\x00", 2)
	
	// Parse the main command: <old-sha> <new-sha> <ref-name>
	fields := strings.Fields(parts[0])
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid command line: %s", data)
	}

	cmd := &ReceivePackCommand{
		OldSHA:  fields[0],
		NewSHA:  fields[1],
		RefName: fields[2],
	}

	// Parse capabilities if present
	if len(parts) > 1 {
		cmd.Capabilities = strings.Split(parts[1], " ")
	}

	return cmd, nil
}

// findPackStart finds the position where pack data starts
// The pack data starts with the "PACK" signature
func findPackStart(body []byte) int {
	// Look for "PACK" signature
	idx := bytes.Index(body, []byte("PACK"))
	return idx
}

// String returns a string representation of the command
func (c *ReceivePackCommand) String() string {
	return fmt.Sprintf("%s %s %s", c.OldSHA, c.NewSHA, c.RefName)
}

// IsCreate returns true if this is a new ref creation (old-sha is all zeros)
func (c *ReceivePackCommand) IsCreate() bool {
	return c.OldSHA == "0000000000000000000000000000000000000000"
}

// IsDelete returns true if this is a ref deletion (new-sha is all zeros)
func (c *ReceivePackCommand) IsDelete() bool {
	return c.NewSHA == "0000000000000000000000000000000000000000"
}
