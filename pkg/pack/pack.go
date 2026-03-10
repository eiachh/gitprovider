package pack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

const (
	// PackSignature is the signature at the start of a pack file
	PackSignature = "PACK"
	
	// PackVersion is the supported pack file version
	PackVersion = 2
)

// ObjectType represents the type of a git object
type ObjectType int

const (
	ObjectCommit    ObjectType = 1
	ObjectTree      ObjectType = 2
	ObjectBlob      ObjectType = 3
	ObjectTag       ObjectType = 4
	ObjectOfsDelta  ObjectType = 6
	ObjectRefDelta  ObjectType = 7
)

// String returns the string representation of the object type
func (t ObjectType) String() string {
	switch t {
	case ObjectCommit:
		return "commit"
	case ObjectTree:
		return "tree"
	case ObjectBlob:
		return "blob"
	case ObjectTag:
		return "tag"
	case ObjectOfsDelta:
		return "ofs_delta"
	case ObjectRefDelta:
		return "ref_delta"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// PackHeader represents the header of a pack file
type PackHeader struct {
	Signature    string
	Version      uint32
	ObjectCount  uint32
}

// PackObject represents a single object in a pack file
type PackObject struct {
	Type     ObjectType
	Size     int64
	Data     []byte
	// For delta objects
	BaseSHA    string // For ref_delta
	BaseOffset int64  // For ofs_delta
}

// PackFile represents a parsed git pack file
type PackFile struct {
	Header  PackHeader
	Objects []*PackObject
}

// ParsePackFile parses a raw pack file
func ParsePackFile(data []byte) (*PackFile, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("pack data too short: %d bytes", len(data))
	}

	reader := bytes.NewReader(data)
	pack := &PackFile{
		Objects: []*PackObject{},
	}

	// Parse header
	if err := parseHeader(reader, &pack.Header); err != nil {
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Parse objects
	for i := uint32(0); i < pack.Header.ObjectCount; i++ {
		obj, err := parseObject(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to parse object %d: %w", i, err)
		}
		pack.Objects = append(pack.Objects, obj)
	}

	return pack, nil
}

// parseHeader reads and validates the pack file header
func parseHeader(reader *bytes.Reader, header *PackHeader) error {
	// Read signature (4 bytes)
	sig := make([]byte, 4)
	if _, err := io.ReadFull(reader, sig); err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}
	header.Signature = string(sig)
	
	if header.Signature != PackSignature {
		return fmt.Errorf("invalid pack signature: %q", header.Signature)
	}

	// Read version (4 bytes, big-endian)
	var version uint32
	if err := binary.Read(reader, binary.BigEndian, &version); err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	header.Version = version

	if header.Version != PackVersion {
		return fmt.Errorf("unsupported pack version: %d", header.Version)
	}

	// Read object count (4 bytes, big-endian)
	var count uint32
	if err := binary.Read(reader, binary.BigEndian, &count); err != nil {
		return fmt.Errorf("failed to read object count: %w", err)
	}
	header.ObjectCount = count

	return nil
}

// parseObject reads a single object from the pack
func parseObject(reader *bytes.Reader) (*PackObject, error) {
	obj := &PackObject{}

	// Read type and size (variable length encoding)
	objType, size, err := readTypeAndSize(reader)
	if err != nil {
		return nil, err
	}
	obj.Type = objType
	obj.Size = size

	// Handle delta base reference
	switch obj.Type {
	case ObjectOfsDelta:
		offset, err := readOffsetEncoding(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read ofs_delta offset: %w", err)
		}
		obj.BaseOffset = offset
	case ObjectRefDelta:
		sha := make([]byte, 20)
		if _, err := io.ReadFull(reader, sha); err != nil {
			return nil, fmt.Errorf("failed to read ref_delta base SHA: %w", err)
		}
		obj.BaseSHA = fmt.Sprintf("%x", sha)
	}

	// Read and decompress the data
	data, err := readCompressedData(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}
	obj.Data = data

	return obj, nil
}

// readTypeAndSize reads the object type and size using variable length encoding
func readTypeAndSize(reader *bytes.Reader) (ObjectType, int64, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return 0, 0, err
	}

	// First byte: bits 6-4 are type (3 bits), bits 3-0 are size (4 bits)
	objType := ObjectType((b >> 4) & 0x07)
	size := int64(b & 0x0F)
	shift := uint(4)

	// If bit 7 is set, continue reading size
	for b&0x80 != 0 {
		b, err = reader.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		size |= int64(b&0x7F) << shift
		shift += 7
	}

	return objType, size, nil
}

// readOffsetEncoding reads the offset for ofs_delta objects
func readOffsetEncoding(reader *bytes.Reader) (int64, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return 0, err
	}

	offset := int64(b & 0x7F)
	
	for b&0x80 != 0 {
		b, err = reader.ReadByte()
		if err != nil {
			return 0, err
		}
		offset = ((offset + 1) << 7) | int64(b&0x7F)
	}

	return offset, nil
}

// readCompressedData reads zlib-compressed data until the end of the stream
func readCompressedData(reader *bytes.Reader) ([]byte, error) {
	// We need to decompress the zlib data
	// Since we don't want to import compress/zlib for now, we'll read until
	// we hit the next object or end of file
	
	// For a proper implementation, we would use zlib.NewReader
	// For now, we'll store the raw compressed data position
	// and skip actual decompression
	
	// This is a simplified version that tries to read until decompression ends
	// A proper implementation would use compress/zlib
	
	return readZlibData(reader)
}

// readZlibData reads zlib compressed data using standard library
func readZlibData(reader *bytes.Reader) ([]byte, error) {
	// Get current position to potentially restore on error
	pos, _ := reader.Seek(0, io.SeekCurrent)
	
	// Create a zlib reader from the current position
	zlibReader, err := zlib.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer zlibReader.Close()
	
	// Read all decompressed data
	data, err := io.ReadAll(zlibReader)
	if err != nil {
		// Restore position and return error
		reader.Seek(pos, io.SeekStart)
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	
	return data, nil
}

// String returns a summary of the pack file
func (p *PackFile) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pack File:\n"))
	sb.WriteString(fmt.Sprintf("  Signature: %s\n", p.Header.Signature))
	sb.WriteString(fmt.Sprintf("  Version: %d\n", p.Header.Version))
	sb.WriteString(fmt.Sprintf("  Objects: %d\n", p.Header.ObjectCount))
	
	for i, obj := range p.Objects {
		sb.WriteString(fmt.Sprintf("  Object %d: type=%s size=%d", i, obj.Type, obj.Size))
		if obj.BaseSHA != "" {
			sb.WriteString(fmt.Sprintf(" base_sha=%s", obj.BaseSHA[:8]))
		}
		if obj.BaseOffset > 0 {
			sb.WriteString(fmt.Sprintf(" base_offset=%d", obj.BaseOffset))
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}
