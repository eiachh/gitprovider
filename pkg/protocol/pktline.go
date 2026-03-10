package protocol

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// PktLine represents a git pkt-line
type PktLine struct {
	Length int
	Data   []byte
}

// EncodePktLine encodes a string as a git pkt-line
func EncodePktLine(s string) []byte {
	length := len(s) + 4
	return []byte(fmt.Sprintf("%04x%s", length, s))
}

// EncodeFlush returns a flush packet
func EncodeFlush() []byte {
	return []byte("0000")
}

// DecodePktLine decodes a single pkt-line from the reader
func DecodePktLine(reader *bufio.Reader) (*PktLine, error) {
	// Read the 4-byte length prefix
	lenBytes := make([]byte, 4)
	_, err := io.ReadFull(reader, lenBytes)
	if err != nil {
		return nil, err
	}

	// Parse the hex length
	var length int
	_, err = fmt.Sscanf(string(lenBytes), "%04x", &length)
	if err != nil {
		return nil, fmt.Errorf("invalid pkt-line length: %s", string(lenBytes))
	}

	// Flush packet
	if length == 0 {
		return &PktLine{Length: 0, Data: nil}, nil
	}

	// Read the content (length includes the 4-byte prefix)
	content := make([]byte, length-4)
	_, err = io.ReadFull(reader, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read pkt-line content: %w", err)
	}

	return &PktLine{Length: length, Data: content}, nil
}

// ReadAllPktLines reads all pkt-lines from the data until a flush packet or error
func ReadAllPktLines(data []byte) ([]*PktLine, error) {
	var lines []*PktLine
	reader := bufio.NewReader(bytes.NewReader(data))

	for {
		line, err := DecodePktLine(reader)
		if err != nil {
			break
		}
		if line.Length == 0 {
			break // Flush packet
		}
		lines = append(lines, line)
	}

	return lines, nil
}
