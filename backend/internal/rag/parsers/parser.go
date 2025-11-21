package parsers

import "io"

// Parser defines the interface for document parsers
type Parser interface {
	// Parse reads from the reader and extracts text content
	Parse(reader io.Reader) (string, error)
	
	// SupportedExtensions returns the list of supported file extensions (e.g. ".txt")
	SupportedExtensions() []string
	
	// CanParse checks if the parser supports the given extension
	CanParse(extension string) bool
}
