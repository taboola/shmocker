// Package dockerfile provides lexical analysis for Dockerfile parsing.
package dockerfile

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
)

// TokenType represents the type of a lexical token.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenNewline
	TokenComment
	TokenDirective
	TokenInstruction
	TokenArgument
	TokenFlag
	TokenString
	TokenHereDoc
	TokenLineContinuation
	TokenError
)

// Token represents a lexical token.
type Token struct {
	Type     TokenType
	Value    string
	Line     int
	Column   int
	StartPos int
	EndPos   int
}

// Lexer tokenizes Dockerfile content.
type Lexer struct {
	input      *bufio.Scanner
	current    rune
	position   int  // current position in input (points to current char)
	readPos    int  // current reading position in input (after current char)
	line       int  // current line number
	column     int  // current column number
	lineStart  int  // position where current line starts
	escapeChar rune // escape character from directive (default '\')
	
	// Buffer for building tokens
	buf strings.Builder
	
	// Source content for position tracking
	source string
	sourceLines []string
}

// NewLexer creates a new lexer for the given input.
func NewLexer(r io.Reader) (*Lexer, error) {
	// Read all content first to support position tracking
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	
	source := string(content)
	lines := strings.Split(source, "\n")
	
	l := &Lexer{
		input:       bufio.NewScanner(strings.NewReader(source)),
		line:        1,
		column:      1,
		escapeChar:  '\\',
		source:      source,
		sourceLines: lines,
	}
	
	// Read first character
	l.readChar()
	return l, nil
}

// readChar reads the next character and advances position.
func (l *Lexer) readChar() {
	if l.readPos >= len(l.source) {
		l.current = 0 // ASCII NUL represents EOF
	} else {
		l.current = rune(l.source[l.readPos])
	}
	l.position = l.readPos
	l.readPos++
	
	if l.current == '\n' {
		l.line++
		l.lineStart = l.position + 1
		l.column = 1
	} else {
		l.column++
	}
}

// peekChar returns the next character without advancing position.
func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.source) {
		return 0
	}
	return rune(l.source[l.readPos])
}

// peekString returns the next n characters without advancing position.
func (l *Lexer) peekString(n int) string {
	start := l.readPos
	end := start + n
	if end > len(l.source) {
		end = len(l.source)
	}
	return l.source[start:end]
}

// skipWhitespace skips whitespace characters except newlines.
func (l *Lexer) skipWhitespace() {
	for l.current != 0 && (l.current == ' ' || l.current == '\t' || l.current == '\r') {
		l.readChar()
	}
}

// readLine reads characters until newline or EOF.
func (l *Lexer) readLine() string {
	l.buf.Reset()
	for l.current != 0 && l.current != '\n' {
		l.buf.WriteRune(l.current)
		l.readChar()
	}
	return l.buf.String()
}

// readString reads a quoted string, handling escape sequences.
func (l *Lexer) readString(quote rune) string {
	l.buf.Reset()
	l.readChar() // skip opening quote
	
	for l.current != 0 && l.current != quote {
		if l.current == l.escapeChar {
			l.readChar() // skip escape char
			if l.current != 0 {
				// Handle common escape sequences
				switch l.current {
				case 'n':
					l.buf.WriteRune('\n')
				case 't':
					l.buf.WriteRune('\t')
				case 'r':
					l.buf.WriteRune('\r')
				case '\\':
					l.buf.WriteRune('\\')
				case '"':
					l.buf.WriteRune('"')
				case '\'':
					l.buf.WriteRune('\'')
				default:
					l.buf.WriteRune(l.current)
				}
				l.readChar()
			}
		} else {
			l.buf.WriteRune(l.current)
			l.readChar()
		}
	}
	
	if l.current == quote {
		l.readChar() // skip closing quote
	}
	
	return l.buf.String()
}

// readWord reads an unquoted word.
func (l *Lexer) readWord() string {
	l.buf.Reset()
	
	// Handle JSON arrays specially
	if l.current == '[' {
		return l.readJSONArray()
	}
	
	for l.current != 0 && !unicode.IsSpace(l.current) && l.current != '#' {
		l.buf.WriteRune(l.current)
		l.readChar()
	}
	return l.buf.String()
}

// readJSONArray reads a JSON array token
func (l *Lexer) readJSONArray() string {
	l.buf.Reset()
	depth := 0
	
	for l.current != 0 {
		l.buf.WriteRune(l.current)
		if l.current == '[' {
			depth++
		} else if l.current == ']' {
			depth--
			if depth == 0 {
				l.readChar() // consume closing bracket
				break
			}
		}
		l.readChar()
	}
	
	return l.buf.String()
}

// readInstruction reads a Dockerfile instruction name.
func (l *Lexer) readInstruction() string {
	l.buf.Reset()
	for l.current != 0 && (unicode.IsLetter(l.current) || unicode.IsDigit(l.current)) {
		l.buf.WriteRune(unicode.ToUpper(l.current))
		l.readChar()
	}
	return l.buf.String()
}

// readHereDoc reads a here-document.
func (l *Lexer) readHereDoc(delimiter string) string {
	l.buf.Reset()
	lines := []string{}
	
	// Skip to next line
	if l.current == '\n' {
		l.readChar()
	}
	
	for {
		line := l.readLine()
		if l.current == '\n' {
			l.readChar()
		}
		
		// Check if this line is the delimiter
		if strings.TrimSpace(line) == delimiter {
			break
		}
		
		lines = append(lines, line)
		
		if l.current == 0 {
			break
		}
	}
	
	return strings.Join(lines, "\n")
}

// isValidInstructionChar checks if character is valid in instruction name.
func isValidInstructionChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() *Token {
	var tok *Token
	
	l.skipWhitespace()
	
	// Track token start position
	startLine := l.line
	startColumn := l.column
	startPos := l.position
	
	switch l.current {
	case 0:
		tok = &Token{Type: TokenEOF, Value: "", Line: startLine, Column: startColumn}
	case '\n':
		tok = &Token{Type: TokenNewline, Value: "\n", Line: startLine, Column: startColumn}
		l.readChar()
	case '#':
		// Handle comments and directives
		// Read the entire line including the '#'
		l.buf.Reset()
		for l.current != 0 && l.current != '\n' {
			l.buf.WriteRune(l.current)
			l.readChar()
		}
		fullLine := l.buf.String()
		
		// Check if it's a parser directive
		trimmed := strings.TrimSpace(fullLine)
		if strings.HasPrefix(trimmed, "# syntax=") ||
		   strings.HasPrefix(trimmed, "# escape=") {
			tok = &Token{Type: TokenDirective, Value: trimmed, Line: startLine, Column: startColumn}
			
			// Handle escape directive
			if strings.HasPrefix(trimmed, "# escape=") {
				escapeStr := strings.TrimPrefix(trimmed, "# escape=")
				if len(escapeStr) > 0 {
					l.escapeChar = rune(escapeStr[0])
				}
			}
		} else {
			tok = &Token{Type: TokenComment, Value: trimmed, Line: startLine, Column: startColumn}
		}
	case '"':
		value := l.readString('"')
		tok = &Token{Type: TokenString, Value: value, Line: startLine, Column: startColumn}
	case '\'':
		value := l.readString('\'')
		tok = &Token{Type: TokenString, Value: value, Line: startLine, Column: startColumn}
	default:
		if l.current == l.escapeChar {
			// Check for line continuation
			if l.peekChar() == '\n' {
				l.readChar() // skip escape char
				l.readChar() // skip newline
				tok = &Token{Type: TokenLineContinuation, Value: string(l.escapeChar) + "\n", Line: startLine, Column: startColumn}
			} else {
				// Regular argument
				word := l.readWord()
				tok = &Token{Type: TokenArgument, Value: word, Line: startLine, Column: startColumn}
			}
		} else if unicode.IsLetter(l.current) {
			// Check if we're at the start of a line (potential instruction)
			if startColumn == 1 || (startColumn > 1 && l.isLineStart(startPos)) {
				instruction := l.readInstruction()
				if isValidInstruction(instruction) {
					tok = &Token{Type: TokenInstruction, Value: instruction, Line: startLine, Column: startColumn}
				} else {
					tok = &Token{Type: TokenArgument, Value: instruction, Line: startLine, Column: startColumn}
				}
			} else {
				word := l.readWord()
				tok = &Token{Type: TokenArgument, Value: word, Line: startLine, Column: startColumn}
			}
		} else if l.current == '-' && l.peekChar() == '-' {
			// Handle flags
			flag := l.readFlag()
			tok = &Token{Type: TokenFlag, Value: flag, Line: startLine, Column: startColumn}
		} else {
			word := l.readWord()
			if word == "" {
				tok = &Token{Type: TokenError, Value: fmt.Sprintf("unexpected character: %c", l.current), Line: startLine, Column: startColumn}
				l.readChar()
			} else {
				tok = &Token{Type: TokenArgument, Value: word, Line: startLine, Column: startColumn}
			}
		}
	}
	
	if tok != nil {
		tok.StartPos = startPos
		tok.EndPos = l.position
	}
	
	return tok
}

// readFlag reads a command flag (--flag=value).
func (l *Lexer) readFlag() string {
	l.buf.Reset()
	// Read --
	l.buf.WriteRune(l.current)
	l.readChar()
	l.buf.WriteRune(l.current)
	l.readChar()
	
	// Read flag name and value
	for l.current != 0 && !unicode.IsSpace(l.current) && l.current != '#' {
		l.buf.WriteRune(l.current)
		l.readChar()
	}
	
	return l.buf.String()
}

// isLineStart checks if position is at the start of a line (ignoring whitespace).
func (l *Lexer) isLineStart(pos int) bool {
	// Go back to start of line and check if there are only whitespace chars
	lineStart := l.lineStart
	for i := lineStart; i < pos; i++ {
		if i >= len(l.source) {
			break
		}
		ch := l.source[i]
		if ch != ' ' && ch != '\t' && ch != '\r' {
			return false
		}
	}
	return true
}

// isValidInstruction checks if a string is a valid Dockerfile instruction.
func isValidInstruction(s string) bool {
	validInstructions := map[string]bool{
		"FROM":        true,
		"RUN":         true,
		"CMD":         true,
		"LABEL":       true,
		"EXPOSE":      true,
		"ENV":         true,
		"ADD":         true,
		"COPY":        true,
		"ENTRYPOINT":  true,
		"VOLUME":      true,
		"USER":        true,
		"WORKDIR":     true,
		"ARG":         true,
		"ONBUILD":     true,
		"STOPSIGNAL":  true,
		"HEALTHCHECK": true,
		"SHELL":       true,
	}
	return validInstructions[s]
}

// TokenizeAll returns all tokens from the input.
func (l *Lexer) TokenizeAll() ([]*Token, error) {
	var tokens []*Token
	
	for {
		tok := l.NextToken()
		if tok == nil {
			break
		}
		
		if tok.Type == TokenError {
			return nil, fmt.Errorf("lexer error at line %d, column %d: %s", tok.Line, tok.Column, tok.Value)
		}
		
		tokens = append(tokens, tok)
		
		if tok.Type == TokenEOF {
			break
		}
	}
	
	return tokens, nil
}

// String returns a string representation of the token type.
func (tt TokenType) String() string {
	switch tt {
	case TokenEOF:
		return "EOF"
	case TokenNewline:
		return "NEWLINE"
	case TokenComment:
		return "COMMENT"
	case TokenDirective:
		return "DIRECTIVE"
	case TokenInstruction:
		return "INSTRUCTION"
	case TokenArgument:
		return "ARGUMENT"
	case TokenFlag:
		return "FLAG"
	case TokenString:
		return "STRING"
	case TokenHereDoc:
		return "HEREDOC"
	case TokenLineContinuation:
		return "LINE_CONTINUATION"
	case TokenError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// String returns a string representation of the token.
func (t *Token) String() string {
	return fmt.Sprintf("%s(%q) at %d:%d", t.Type, t.Value, t.Line, t.Column)
}

// expandArgs expands build arguments in token values.
func expandArgs(value string, buildArgs map[string]string) string {
	// Simple ARG expansion - replace ${VAR} and $VAR patterns
	argPattern := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
	
	return argPattern.ReplaceAllStringFunc(value, func(match string) string {
		var varName string
		if strings.HasPrefix(match, "${") {
			// ${VAR} format
			varName = match[2 : len(match)-1]
		} else {
			// $VAR format
			varName = match[1:]
		}
		
		if val, exists := buildArgs[varName]; exists {
			return val
		}
		return match // return original if not found
	})
}