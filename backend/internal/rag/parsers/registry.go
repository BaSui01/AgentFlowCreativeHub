package parsers

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ParserRegistry manages document parsers
type ParserRegistry struct {
	parsers []Parser
}

// NewParserRegistry creates a new registry with default parsers
func NewParserRegistry() *ParserRegistry {
	r := &ParserRegistry{
		parsers: make([]Parser, 0),
	}
	
	// Register default parsers
	r.Register(NewTextParser())
	r.Register(NewPDFParser())
	// Future: r.Register(NewDocxParser())
	
	return r
}

// Register registers a new parser
func (r *ParserRegistry) Register(p Parser) {
	r.parsers = append(r.parsers, p)
}

// Parse chooses the appropriate parser and parses the document
func (r *ParserRegistry) Parse(fileName string, reader io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	
	for _, p := range r.parsers {
		if p.CanParse(ext) {
			return p.Parse(reader)
		}
	}
	
	return "", fmt.Errorf("no parser found for extension: %s", ext)
}
