package dockerfile

import (
	"strings"
	"testing"
)

func TestLexerBasicTokenization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{
			name:  "simple FROM instruction",
			input: "FROM ubuntu:20.04",
			expected: []TokenType{
				TokenInstruction, // FROM
				TokenArgument,    // ubuntu:20.04
				TokenEOF,
			},
		},
		{
			name:  "instruction with flags",
			input: "RUN --mount=type=cache,target=/cache apt-get update",
			expected: []TokenType{
				TokenInstruction, // RUN
				TokenFlag,        // --mount=type=cache,target=/cache
				TokenArgument,    // apt-get
				TokenArgument,    // update
				TokenEOF,
			},
		},
		{
			name:  "multiline with comments",
			input: "FROM ubuntu\n# This is a comment\nRUN echo hello",
			expected: []TokenType{
				TokenInstruction, // FROM
				TokenArgument,    // ubuntu
				TokenNewline,     // \n
				TokenComment,     // # This is a comment
				TokenNewline,     // \n
				TokenInstruction, // RUN
				TokenArgument,    // echo
				TokenArgument,    // hello
				TokenEOF,
			},
		},
		{
			name:  "directive",
			input: "# syntax=docker/dockerfile:1.4",
			expected: []TokenType{
				TokenDirective, // # syntax=docker/dockerfile:1.4
				TokenEOF,
			},
		},
		{
			name:  "quoted strings",
			input: `COPY "file name.txt" '/app/file name.txt'`,
			expected: []TokenType{
				TokenInstruction, // COPY
				TokenString,      // "file name.txt"
				TokenString,      // '/app/file name.txt'
				TokenEOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer, err := NewLexer(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("failed to create lexer: %v", err)
			}

			var tokens []TokenType
			for {
				token := lexer.NextToken()
				if token == nil {
					break
				}
				tokens = append(tokens, token.Type)
				if token.Type == TokenEOF {
					break
				}
			}

			if len(tokens) != len(tt.expected) {
				t.Errorf("expected %d tokens, got %d", len(tt.expected), len(tokens))
				t.Errorf("expected: %v", tt.expected)
				t.Errorf("got: %v", tokens)
				return
			}

			for i, expected := range tt.expected {
				if tokens[i] != expected {
					t.Errorf("token %d: expected %s, got %s", i, expected, tokens[i])
				}
			}
		})
	}
}

func TestLexerTokenValues(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedToken TokenType
		expectedValue string
	}{
		{
			name:          "FROM instruction",
			input:         "FROM ubuntu",
			expectedToken: TokenInstruction,
			expectedValue: "FROM",
		},
		{
			name:          "image argument",
			input:         "FROM ubuntu:20.04",
			expectedToken: TokenArgument,
			expectedValue: "ubuntu:20.04",
		},
		{
			name:          "flag with value",
			input:         "RUN --mount=type=cache,target=/cache echo test",
			expectedToken: TokenFlag,
			expectedValue: "--mount=type=cache,target=/cache",
		},
		{
			name:          "comment",
			input:         "# This is a comment",
			expectedToken: TokenComment,
			expectedValue: "# This is a comment",
		},
		{
			name:          "directive",
			input:         "# syntax=docker/dockerfile:1.4",
			expectedToken: TokenDirective,
			expectedValue: "# syntax=docker/dockerfile:1.4",
		},
		{
			name:          "quoted string with spaces",
			input:         `COPY "file name.txt" /app`,
			expectedToken: TokenString,
			expectedValue: "file name.txt",
		},
		{
			name:          "single quoted string",
			input:         `ENV PATH '/usr/local/bin:$PATH'`,
			expectedToken: TokenString,
			expectedValue: "/usr/local/bin:$PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer, err := NewLexer(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("failed to create lexer: %v", err)
			}

			var foundToken *Token
			for {
				token := lexer.NextToken()
				if token == nil || token.Type == TokenEOF {
					break
				}
				if token.Type == tt.expectedToken {
					foundToken = token
					break
				}
			}

			if foundToken == nil {
				t.Fatalf("expected token type %s not found", tt.expectedToken)
			}

			if foundToken.Value != tt.expectedValue {
				t.Errorf("expected value %q, got %q", tt.expectedValue, foundToken.Value)
			}
		})
	}
}

func TestLexerLineContinuation(t *testing.T) {
	input := `RUN apt-get update && \
    apt-get install -y curl && \
    apt-get clean`

	lexer, err := NewLexer(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	tokens, err := lexer.TokenizeAll()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}

	// Should find line continuation tokens
	var continuationCount int
	for _, token := range tokens {
		if token.Type == TokenLineContinuation {
			continuationCount++
		}
	}

	if continuationCount != 2 {
		t.Errorf("expected 2 line continuation tokens, got %d", continuationCount)
	}
}

func TestLexerEscapeDirective(t *testing.T) {
	input := `# escape=`
	input += "`\nFROM ubuntu\nRUN echo test `\n    && echo continued"

	lexer, err := NewLexer(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	// First token should be escape directive
	token := lexer.NextToken()
	if token.Type != TokenDirective {
		t.Errorf("expected directive token, got %s", token.Type)
	}

	if !strings.Contains(token.Value, "escape=") {
		t.Errorf("expected escape directive, got %q", token.Value)
	}

	// The lexer should now use backtick as escape character
	// This would be verified by checking line continuation behavior
}

func TestLexerSourceLocations(t *testing.T) {
	input := `FROM ubuntu
RUN echo "line 2"
COPY . /app`

	lexer, err := NewLexer(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	tokens, err := lexer.TokenizeAll()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}

	// Check that line numbers are correct
	expectedLines := []int{1, 1, 2, 2, 2, 2, 3, 3, 3}
	var actualLines []int

	for _, token := range tokens {
		if token.Type != TokenNewline && token.Type != TokenEOF {
			actualLines = append(actualLines, token.Line)
		}
	}

	if len(actualLines) != len(expectedLines) {
		t.Errorf("expected %d line numbers, got %d", len(expectedLines), len(actualLines))
		return
	}

	for i, expected := range expectedLines {
		if i < len(actualLines) && actualLines[i] != expected {
			t.Errorf("token %d: expected line %d, got %d", i, expected, actualLines[i])
		}
	}
}

func TestLexerComplexExample(t *testing.T) {
	input := `# syntax=docker/dockerfile:1.4
# Build stage
FROM --platform=linux/amd64 golang:1.19 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o app .

# Runtime stage  
FROM alpine:3.16
RUN apk --no-cache add ca-certificates
COPY --from=builder /src/app /usr/local/bin/app
ENTRYPOINT ["app"]`

	lexer, err := NewLexer(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	tokens, err := lexer.TokenizeAll()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}

	// Verify we got tokens
	if len(tokens) == 0 {
		t.Fatal("expected tokens but got none")
	}

	// Count instruction types
	instructionCount := 0
	flagCount := 0
	commentCount := 0
	directiveCount := 0

	for _, token := range tokens {
		switch token.Type {
		case TokenInstruction:
			instructionCount++
		case TokenFlag:
			flagCount++
		case TokenComment:
			commentCount++
		case TokenDirective:
			directiveCount++
		}
	}

	if instructionCount == 0 {
		t.Error("expected instruction tokens")
	}
	if flagCount == 0 {
		t.Error("expected flag tokens")
	}
	if commentCount == 0 {
		t.Error("expected comment tokens")
	}
	if directiveCount == 0 {
		t.Error("expected directive tokens")
	}
}

func TestLexerErrorCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "valid input",
			input:     "FROM ubuntu",
			expectErr: false,
		},
		// Add specific error cases if lexer should detect them
		// For now, most malformed input is handled by parser
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer, err := NewLexer(strings.NewReader(tt.input))
			if err != nil {
				if !tt.expectErr {
					t.Errorf("unexpected lexer creation error: %v", err)
				}
				return
			}

			_, err = lexer.TokenizeAll()
			if tt.expectErr && err == nil {
				t.Error("expected tokenization error but got none")
			} else if !tt.expectErr && err != nil {
				t.Errorf("unexpected tokenization error: %v", err)
			}
		})
	}
}

func TestLexerJSONArrayParsing(t *testing.T) {
	input := `RUN ["apt-get", "update"]`

	lexer, err := NewLexer(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	tokens, err := lexer.TokenizeAll()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}

	// Find the JSON array token
	var jsonToken *Token
	for _, token := range tokens {
		if strings.HasPrefix(token.Value, "[") {
			jsonToken = token
			break
		}
	}

	if jsonToken == nil {
		t.Fatal("expected JSON array token not found")
	}

	if jsonToken.Type != TokenArgument {
		t.Errorf("expected argument token for JSON array, got %s", jsonToken.Type)
	}

	expected := `["apt-get", "update"]`
	if jsonToken.Value != expected {
		t.Errorf("expected %q, got %q", expected, jsonToken.Value)
	}
}

func TestArgExpansionInLexer(t *testing.T) {
	buildArgs := map[string]string{
		"VERSION": "1.0.0",
		"PORT":    "8080",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"${VERSION}", "1.0.0"},
		{"$VERSION", "1.0.0"},
		{"app:${VERSION}", "app:1.0.0"},
		{"port $PORT", "port 8080"},
		{"${UNDEFINED}", "${UNDEFINED}"}, // should remain unchanged
		{"$UNDEFINED", "$UNDEFINED"},     // should remain unchanged
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandArgs(tt.input, buildArgs)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestLexerWhitespaceHandling(t *testing.T) {
	input := "  FROM   ubuntu:20.04  \n\t  RUN   echo   test  "

	lexer, err := NewLexer(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to create lexer: %v", err)
	}

	tokens, err := lexer.TokenizeAll()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}

	// Verify whitespace is properly skipped
	var nonWhitespaceTokens []*Token
	for _, token := range tokens {
		if token.Type != TokenNewline && token.Type != TokenEOF {
			nonWhitespaceTokens = append(nonWhitespaceTokens, token)
		}
	}

	expected := []string{"FROM", "ubuntu:20.04", "RUN", "echo", "test"}
	if len(nonWhitespaceTokens) != len(expected) {
		t.Errorf("expected %d non-whitespace tokens, got %d", len(expected), len(nonWhitespaceTokens))
		return
	}

	for i, expectedValue := range expected {
		if nonWhitespaceTokens[i].Value != expectedValue {
			t.Errorf("token %d: expected %q, got %q", i, expectedValue, nonWhitespaceTokens[i].Value)
		}
	}
}