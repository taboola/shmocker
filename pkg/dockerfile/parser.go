// Package dockerfile provides Dockerfile parsing functionality.
package dockerfile

import (
	"fmt"
	"io"
)

// Dockerfile represents a parsed Dockerfile.
type Dockerfile struct {
	Instructions []Instruction
}

// Instruction represents a single Dockerfile instruction.
type Instruction struct {
	Cmd   string
	Value string
	Args  []string
}

// Parser parses Dockerfile content.
type Parser struct {
	// TODO: Add parser configuration
}

// New creates a new Dockerfile parser.
func New() *Parser {
	return &Parser{}
}

// Parse parses a Dockerfile from the given reader.
func (p *Parser) Parse(r io.Reader) (*Dockerfile, error) {
	// TODO: Implement Dockerfile parsing logic
	return nil, fmt.Errorf("dockerfile parsing not yet implemented")
}