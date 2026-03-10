package protocol

import (
	"fmt"
)

// InfoRefsResponse represents the response for the info/refs endpoint
type InfoRefsResponse struct {
	Service     string
	Capabilities []string
	Refs        map[string]string // ref-name -> SHA
}

// NewInfoRefsResponse creates a new InfoRefsResponse for git-receive-pack
func NewInfoRefsResponse() *InfoRefsResponse {
	return &InfoRefsResponse{
		Service: "git-receive-pack",
		Capabilities: []string{
			"report-status",
			"delete-refs",
			"quiet",
			"atomic",
		},
		Refs: make(map[string]string),
	}
}

// Encode encodes the InfoRefsResponse as pkt-lines
func (r *InfoRefsResponse) Encode() []byte {
	var buf []byte

	// Service announcement
	buf = append(buf, EncodePktLine(fmt.Sprintf("# service=%s\n", r.Service))...)
	buf = append(buf, EncodeFlush()...)

	// Capabilities announcement (using a dummy ref for capabilities)
	// Format: <sha> <ref-name>\0<capabilities>
	capsLine := "0000000000000000000000000000000000000000 capabilities^{}\x00" + 
		stringsJoin(r.Capabilities, " ") + "\n"
	buf = append(buf, EncodePktLine(capsLine)...)

	// Add refs if any
	for refName, sha := range r.Refs {
		line := fmt.Sprintf("%s %s\n", sha, refName)
		buf = append(buf, EncodePktLine(line)...)
	}

	buf = append(buf, EncodeFlush()...)

	return buf
}

// stringsJoin joins strings with a separator (helper to avoid importing strings)
func stringsJoin(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
