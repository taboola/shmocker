// Package dockerfile provides Dockerfile parsing functionality.
package dockerfile

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// ParserImpl implements the Parser interface.
type ParserImpl struct {
	lexer      *Lexer
	tokens     []*Token
	current    int
	buildArgs  map[string]string
	stages     []*Stage
	stageNames map[string]int
}

// New creates a new Dockerfile parser.
func New() Parser {
	return &ParserImpl{
		buildArgs:  make(map[string]string),
		stageNames: make(map[string]int),
	}
}

// Parse parses a Dockerfile from the given reader and returns an AST.
func (p *ParserImpl) Parse(reader io.Reader) (*AST, error) {
	// Initialize lexer
	lexer, err := NewLexer(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create lexer: %w", err)
	}
	
	p.lexer = lexer
	
	// Tokenize input
	tokens, err := lexer.TokenizeAll()
	if err != nil {
		return nil, fmt.Errorf("lexical analysis failed: %w", err)
	}
	
	p.tokens = tokens
	p.current = 0
	p.stages = []*Stage{}
	p.stageNames = make(map[string]int)
	
	// Parse AST
	ast, err := p.parseAST()
	if err != nil {
		return nil, fmt.Errorf("parsing failed: %w", err)
	}
	
	return ast, nil
}

// ParseFile parses a Dockerfile from the specified file path.
func (p *ParserImpl) ParseFile(path string) (*AST, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()
	
	ast, err := p.Parse(file)
	if err != nil {
		return nil, err
	}
	
	// Set filename in metadata
	if ast.Metadata != nil {
		ast.Metadata.Filename = path
	}
	
	return ast, nil
}

// ParseBytes parses a Dockerfile from byte content.
func (p *ParserImpl) ParseBytes(content []byte) (*AST, error) {
	return p.Parse(strings.NewReader(string(content)))
}

// Validate performs syntax validation on a Dockerfile AST.
func (p *ParserImpl) Validate(ast *AST) error {
	if ast == nil {
		return fmt.Errorf("AST is nil")
	}
	
	if len(ast.Stages) == 0 {
		return fmt.Errorf("Dockerfile must contain at least one FROM instruction")
	}
	
	// Validate each stage
	for i, stage := range ast.Stages {
		if err := p.validateStage(stage, i); err != nil {
			return fmt.Errorf("stage %d validation failed: %w", i, err)
		}
	}
	
	// Validate stage name uniqueness
	stageNames := make(map[string]int)
	for i, stage := range ast.Stages {
		if stage.Name != "" {
			if prevIndex, exists := stageNames[stage.Name]; exists {
				return fmt.Errorf("duplicate stage name '%s' at stage %d (previously defined at stage %d)", stage.Name, i, prevIndex)
			}
			stageNames[stage.Name] = i
		}
	}
	
	return nil
}

// parseAST parses the token stream into an AST.
func (p *ParserImpl) parseAST() (*AST, error) {
	ast := &AST{
		Stages:     []*Stage{},
		Directives: []*Directive{},
		Comments:   []*Comment{},
		Metadata: &ParseMetadata{
			ParseTime:     time.Now(),
			ParserVersion: "1.0.0",
			Warnings:      []*ParseWarning{},
		},
	}
	
	var currentStage *Stage
	stageIndex := -1
	
	for !p.isAtEnd() {
		token := p.peek()
		
		switch token.Type {
		case TokenDirective:
			directive, err := p.parseDirective()
			if err != nil {
				return nil, err
			}
			ast.Directives = append(ast.Directives, directive)
			
		case TokenComment:
			comment, err := p.parseComment()
			if err != nil {
				return nil, err
			}
			ast.Comments = append(ast.Comments, comment)
			
		case TokenInstruction:
			if token.Value == "FROM" {
				// Start new stage
				stageIndex++
				stage, err := p.parseStage(stageIndex)
				if err != nil {
					return nil, fmt.Errorf("failed to parse stage %d: %w", stageIndex, err)
				}
				ast.Stages = append(ast.Stages, stage)
				currentStage = stage
			} else {
				// Regular instruction within current stage
				if currentStage == nil {
					return nil, fmt.Errorf("instruction %s found before FROM at line %d", token.Value, token.Line)
				}
				
				instruction, err := p.parseInstruction()
				if err != nil {
					return nil, fmt.Errorf("failed to parse instruction %s at line %d: %w", token.Value, token.Line, err)
				}
				currentStage.Instructions = append(currentStage.Instructions, instruction)
			}
			
		case TokenNewline:
			p.advance() // skip newlines
			
		case TokenEOF:
			break
			
		default:
			return nil, fmt.Errorf("unexpected token %s at line %d", token.Type, token.Line)
		}
	}
	
	// Check if we have any stages after parsing
	if len(ast.Stages) == 0 {
		return nil, fmt.Errorf("Dockerfile must contain at least one FROM instruction")
	}
	
	return ast, nil
}

// parseStage parses a single build stage starting with FROM.
func (p *ParserImpl) parseStage(index int) (*Stage, error) {
	// Parse FROM instruction
	fromInstr, err := p.parseFromInstruction()
	if err != nil {
		return nil, err
	}
	
	stage := &Stage{
		Index:        index,
		From:         fromInstr,
		Instructions: []Instruction{},
		Location:     fromInstr.Location,
	}
	
	// Set stage name if provided
	if fromInstr.As != "" {
		stage.Name = fromInstr.As
		p.stageNames[fromInstr.As] = index
	}
	
	// Set platform if provided
	if fromInstr.Platform != "" {
		stage.Platform = fromInstr.Platform
	}
	
	return stage, nil
}

// parseDirective parses a parser directive.
func (p *ParserImpl) parseDirective() (*Directive, error) {
	token := p.advance()
	
	// Parse directive format: # name=value
	parts := strings.SplitN(strings.TrimPrefix(token.Value, "#"), "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid directive format at line %d", token.Line)
	}
	
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	
	directive := &Directive{
		Name:  name,
		Value: value,
		Location: &SourceLocation{
			Line:   token.Line,
			Column: token.Column,
		},
	}
	
	// Handle special directives
	switch name {
	case "syntax":
		if p.getCurrentAST() != nil && p.getCurrentAST().Metadata != nil {
			p.getCurrentAST().Metadata.Syntax = value
		}
	case "escape":
		// Escape character is already handled by lexer
	}
	
	return directive, nil
}

// parseComment parses a comment.
func (p *ParserImpl) parseComment() (*Comment, error) {
	token := p.advance()
	
	comment := &Comment{
		Text: strings.TrimPrefix(token.Value, "#"),
		Location: &SourceLocation{
			Line:   token.Line,
			Column: token.Column,
		},
	}
	
	return comment, nil
}

// parseInstruction parses a single instruction.
func (p *ParserImpl) parseInstruction() (Instruction, error) {
	token := p.peek()
	
	switch token.Value {
	case "FROM":
		return p.parseFromInstruction()
	case "RUN":
		return p.parseRunInstruction()
	case "CMD":
		return p.parseCmdInstruction()
	case "COPY":
		return p.parseCopyInstruction()
	case "ADD":
		return p.parseAddInstruction()
	case "ENV":
		return p.parseEnvInstruction()
	case "ENTRYPOINT":
		return p.parseEntrypointInstruction()
	case "WORKDIR":
		return p.parseWorkdirInstruction()
	case "USER":
		return p.parseUserInstruction()
	case "VOLUME":
		return p.parseVolumeInstruction()
	case "EXPOSE":
		return p.parseExposeInstruction()
	case "LABEL":
		return p.parseLabelInstruction()
	case "ARG":
		return p.parseArgInstruction()
	case "ONBUILD":
		return p.parseOnbuildInstruction()
	case "STOPSIGNAL":
		return p.parseStopsignalInstruction()
	case "HEALTHCHECK":
		return p.parseHealthcheckInstruction()
	case "SHELL":
		return p.parseShellInstruction()
	default:
		return nil, fmt.Errorf("unknown instruction: %s at line %d", token.Value, token.Line)
	}
}

// parseFromInstruction parses a FROM instruction.
func (p *ParserImpl) parseFromInstruction() (*FromInstruction, error) {
	startToken := p.advance() // consume FROM
	
	instr := &FromInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse flags
	for p.peek().Type == TokenFlag {
		flag := p.advance()
		if err := p.parseFlag(flag.Value, instr); err != nil {
			return nil, err
		}
	}
	
	// Parse image reference
	if p.peek().Type != TokenArgument && p.peek().Type != TokenString {
		return nil, fmt.Errorf("FROM instruction requires an image argument at line %d", p.peek().Line)
	}
	
	imageRef := p.advance()
	imageStr := p.expandBuildArgs(imageRef.Value)
	
	// Parse image reference parts
	if err := p.parseImageReference(imageStr, instr); err != nil {
		return nil, fmt.Errorf("invalid image reference '%s' at line %d: %w", imageStr, imageRef.Line, err)
	}
	
	// Parse AS clause
	if p.peek().Type == TokenArgument && strings.ToUpper(p.peek().Value) == "AS" {
		p.advance() // consume AS
		if p.peek().Type != TokenArgument && p.peek().Type != TokenString {
			return nil, fmt.Errorf("FROM AS requires a stage name at line %d", p.peek().Line)
		}
		stageNameToken := p.advance()
		instr.As = stageNameToken.Value
	}
	
	return instr, nil
}

// parseRunInstruction parses a RUN instruction.
func (p *ParserImpl) parseRunInstruction() (*RunInstruction, error) {
	startToken := p.advance() // consume RUN
	
	instr := &RunInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
		Mounts: []*MountInstruction{},
	}
	
	// Parse flags - only consume RUN instruction flags
	for p.peek().Type == TokenFlag {
		flagToken := p.peek()
		// Check if this is a valid RUN instruction flag
		if isValidRunFlag(flagToken.Value) {
			flag := p.advance()
			if err := p.parseRunFlag(flag.Value, instr); err != nil {
				return nil, err
			}
		} else {
			// This flag is part of the command, not an instruction flag
			break
		}
	}
	
	// Parse command
	commands, shell, err := p.parseCommand()
	if err != nil {
		return nil, err
	}
	
	instr.Commands = commands
	instr.Shell = shell
	
	return instr, nil
}

// parseCmdInstruction parses a CMD instruction.
func (p *ParserImpl) parseCmdInstruction() (*CmdInstruction, error) {
	startToken := p.advance() // consume CMD
	
	instr := &CmdInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse command
	commands, shell, err := p.parseCommand()
	if err != nil {
		return nil, err
	}
	
	instr.Commands = commands
	instr.Shell = shell
	
	return instr, nil
}

// parseCopyInstruction parses a COPY instruction.
func (p *ParserImpl) parseCopyInstruction() (*CopyInstruction, error) {
	startToken := p.advance() // consume COPY
	
	instr := &CopyInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse flags
	for p.peek().Type == TokenFlag {
		flag := p.advance()
		if err := p.parseCopyFlag(flag.Value, instr); err != nil {
			return nil, err
		}
	}
	
	// Parse source and destination
	sources, dest, err := p.parseSourcesAndDest()
	if err != nil {
		return nil, err
	}
	
	instr.Sources = sources
	instr.Destination = dest
	
	return instr, nil
}

// parseAddInstruction parses an ADD instruction.
func (p *ParserImpl) parseAddInstruction() (*AddInstruction, error) {
	startToken := p.advance() // consume ADD
	
	instr := &AddInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse flags
	for p.peek().Type == TokenFlag {
		flag := p.advance()
		if err := p.parseAddFlag(flag.Value, instr); err != nil {
			return nil, err
		}
	}
	
	// Parse source and destination
	sources, dest, err := p.parseSourcesAndDest()
	if err != nil {
		return nil, err
	}
	
	instr.Sources = sources
	instr.Destination = dest
	
	return instr, nil
}

// parseEnvInstruction parses an ENV instruction.
func (p *ParserImpl) parseEnvInstruction() (*EnvInstruction, error) {
	startToken := p.advance() // consume ENV
	
	instr := &EnvInstruction{
		Variables: make(map[string]string),
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse key-value pairs
	for !p.isAtEnd() && p.peek().Type != TokenNewline && p.peek().Type != TokenInstruction {
		token := p.peek()
		switch token.Type {
		case TokenLineContinuation:
			// Skip line continuation tokens
			p.advance()
			continue
		case TokenArgument, TokenString:
			// Process the argument
			arg := p.advance()
		
		// Check if it's key=value format
		if strings.Contains(arg.Value, "=") {
			parts := strings.SplitN(arg.Value, "=", 2)
			key := parts[0]
			value := p.expandBuildArgs(parts[1])
			instr.Variables[key] = value
		} else {
			// Space-separated format: key value
			key := arg.Value
			if p.peek().Type == TokenArgument || p.peek().Type == TokenString {
				valueToken := p.advance()
				value := p.expandBuildArgs(valueToken.Value)
				instr.Variables[key] = value
			} else {
				return nil, fmt.Errorf("ENV instruction requires a value for key '%s' at line %d", key, token.Line)
			}
		}
		default:
			break
		}
	}
	
	return instr, nil
}

// Helper methods for parsing...

// parseCommand parses a command (for RUN, CMD, ENTRYPOINT).
func (p *ParserImpl) parseCommand() ([]string, bool, error) {
	var commands []string
	shell := true
	
	// Check if it's JSON array format
	if p.peek().Type == TokenArgument && strings.HasPrefix(p.peek().Value, "[") {
		// JSON array format
		shell = false
		jsonStr := p.advance().Value
		
		// Parse JSON array manually (simplified)
		jsonStr = strings.Trim(jsonStr, "[]")
		if jsonStr != "" {
			parts := strings.Split(jsonStr, ",")
			for _, part := range parts {
				part = strings.Trim(part, `"' `)
				if part != "" {
					commands = append(commands, p.expandBuildArgs(part))
				}
			}
		}
	} else {
		// Shell format - collect all remaining arguments
		for !p.isAtEnd() && p.peek().Type != TokenNewline && p.peek().Type != TokenInstruction {
			token := p.peek()
			switch token.Type {
			case TokenArgument, TokenString:
				arg := p.advance()
				commands = append(commands, p.expandBuildArgs(arg.Value))
			case TokenFlag:
				// Command flags like --no-cache should be treated as arguments
				arg := p.advance()
				commands = append(commands, p.expandBuildArgs(arg.Value))
			case TokenLineContinuation:
				// Skip line continuation tokens - they're just formatting
				p.advance()
			default:
				break
			}
		}
	}
	
	return commands, shell, nil
}

// parseSourcesAndDest parses sources and destination for COPY/ADD.
func (p *ParserImpl) parseSourcesAndDest() ([]string, string, error) {
	var sources []string
	var dest string
	
	// Collect all arguments
	var args []string
	for !p.isAtEnd() && p.peek().Type != TokenNewline && p.peek().Type != TokenInstruction {
		token := p.peek()
		if token.Type != TokenArgument && token.Type != TokenString {
			break
		}
		arg := p.advance()
		args = append(args, p.expandBuildArgs(arg.Value))
	}
	
	if len(args) < 2 {
		return nil, "", fmt.Errorf("COPY/ADD instruction requires at least 2 arguments")
	}
	
	// Last argument is destination
	dest = args[len(args)-1]
	sources = args[:len(args)-1]
	
	return sources, dest, nil
}

// Additional instruction parsers (continuing the pattern)...

func (p *ParserImpl) parseEntrypointInstruction() (*EntrypointInstruction, error) {
	startToken := p.advance()
	
	instr := &EntrypointInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	commands, shell, err := p.parseCommand()
	if err != nil {
		return nil, err
	}
	
	instr.Commands = commands
	instr.Shell = shell
	
	return instr, nil
}

func (p *ParserImpl) parseWorkdirInstruction() (*WorkdirInstruction, error) {
	startToken := p.advance()
	
	if p.peek().Type != TokenArgument && p.peek().Type != TokenString {
		return nil, fmt.Errorf("WORKDIR instruction requires a path argument at line %d", p.peek().Line)
	}
	
	pathToken := p.advance()
	
	return &WorkdirInstruction{
		Path: p.expandBuildArgs(pathToken.Value),
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}, nil
}

func (p *ParserImpl) parseUserInstruction() (*UserInstruction, error) {
	startToken := p.advance()
	
	if p.peek().Type != TokenArgument && p.peek().Type != TokenString {
		return nil, fmt.Errorf("USER instruction requires a user argument at line %d", p.peek().Line)
	}
	
	userToken := p.advance()
	userStr := userToken.Value
	
	instr := &UserInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse user:group format
	if strings.Contains(userStr, ":") {
		parts := strings.SplitN(userStr, ":", 2)
		instr.User = p.expandBuildArgs(parts[0])
		instr.Group = p.expandBuildArgs(parts[1])
	} else {
		instr.User = p.expandBuildArgs(userStr)
	}
	
	return instr, nil
}

func (p *ParserImpl) parseVolumeInstruction() (*VolumeInstruction, error) {
	startToken := p.advance()
	
	instr := &VolumeInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse paths
	for !p.isAtEnd() && p.peek().Type != TokenNewline && p.peek().Type != TokenInstruction {
		token := p.peek()
		switch token.Type {
		case TokenLineContinuation:
			// Skip line continuation tokens
			p.advance()
			continue
		case TokenArgument, TokenString:
			pathToken := p.advance()
			instr.Paths = append(instr.Paths, p.expandBuildArgs(pathToken.Value))
		default:
			break
		}
	}
	
	return instr, nil
}

func (p *ParserImpl) parseExposeInstruction() (*ExposeInstruction, error) {
	startToken := p.advance()
	
	instr := &ExposeInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse ports
	for !p.isAtEnd() && p.peek().Type != TokenNewline && p.peek().Type != TokenInstruction {
		token := p.peek()
		switch token.Type {
		case TokenLineContinuation:
			// Skip line continuation tokens
			p.advance()
			continue
		case TokenArgument, TokenString:
			portToken := p.advance()
			instr.Ports = append(instr.Ports, p.expandBuildArgs(portToken.Value))
		default:
			break
		}
	}
	
	return instr, nil
}

func (p *ParserImpl) parseLabelInstruction() (*LabelInstruction, error) {
	startToken := p.advance()
	
	instr := &LabelInstruction{
		Labels: make(map[string]string),
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse key-value pairs (similar to ENV)
	for !p.isAtEnd() && p.peek().Type != TokenNewline && p.peek().Type != TokenInstruction {
		token := p.peek()
		switch token.Type {
		case TokenLineContinuation:
			// Skip line continuation tokens
			p.advance()
			continue
		case TokenArgument, TokenString:
			// Process the argument
			arg := p.advance()
		
			if strings.Contains(arg.Value, "=") {
				parts := strings.SplitN(arg.Value, "=", 2)
				key := parts[0]
				value := p.expandBuildArgs(parts[1])
				instr.Labels[key] = value
			} else {
				key := arg.Value
				if p.peek().Type == TokenArgument || p.peek().Type == TokenString {
					valueToken := p.advance()
					value := p.expandBuildArgs(valueToken.Value)
					instr.Labels[key] = value
				} else {
					return nil, fmt.Errorf("LABEL instruction requires a value for key '%s' at line %d", key, token.Line)
				}
			}
		default:
			break
		}
	}
	
	return instr, nil
}

func (p *ParserImpl) parseArgInstruction() (*ArgInstruction, error) {
	startToken := p.advance()
	
	if p.peek().Type != TokenArgument && p.peek().Type != TokenString {
		return nil, fmt.Errorf("ARG instruction requires an argument at line %d", p.peek().Line)
	}
	
	argToken := p.advance()
	argStr := argToken.Value
	
	instr := &ArgInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse name=default format
	if strings.Contains(argStr, "=") {
		parts := strings.SplitN(argStr, "=", 2)
		instr.Name = parts[0]
		instr.DefaultValue = p.expandBuildArgs(parts[1])
	} else {
		instr.Name = argStr
	}
	
	// Add to build args if it has default value
	if instr.DefaultValue != "" {
		p.buildArgs[instr.Name] = instr.DefaultValue
	}
	
	return instr, nil
}

func (p *ParserImpl) parseOnbuildInstruction() (*OnbuildInstruction, error) {
	startToken := p.advance()
	
	// Parse the sub-instruction
	subInstr, err := p.parseInstruction()
	if err != nil {
		return nil, fmt.Errorf("ONBUILD requires a valid sub-instruction: %w", err)
	}
	
	return &OnbuildInstruction{
		Instruction: subInstr,
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}, nil
}

func (p *ParserImpl) parseStopsignalInstruction() (*StopsignalInstruction, error) {
	startToken := p.advance()
	
	if p.peek().Type != TokenArgument && p.peek().Type != TokenString {
		return nil, fmt.Errorf("STOPSIGNAL instruction requires a signal argument at line %d", p.peek().Line)
	}
	
	signalToken := p.advance()
	
	return &StopsignalInstruction{
		Signal: p.expandBuildArgs(signalToken.Value),
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}, nil
}

func (p *ParserImpl) parseHealthcheckInstruction() (*HealthcheckInstruction, error) {
	startToken := p.advance()
	
	instr := &HealthcheckInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse flags first
	for p.peek().Type == TokenFlag {
		flag := p.advance()
		if err := p.parseHealthcheckFlag(flag.Value, instr); err != nil {
			return nil, err
		}
	}
	
	// Skip line continuation tokens
	for p.peek().Type == TokenLineContinuation {
		p.advance()
	}
	
	// Parse type (NONE or CMD)
	if p.peek().Type != TokenArgument {
		return nil, fmt.Errorf("HEALTHCHECK instruction requires NONE or CMD at line %d", p.peek().Line)
	}
	
	typeToken := p.advance()
	instr.Type = strings.ToUpper(typeToken.Value)
	
	// Parse command if CMD type
	if instr.Type == "CMD" {
		commands, _, err := p.parseCommand()
		if err != nil {
			return nil, err
		}
		instr.Commands = commands
	}
	
	return instr, nil
}

func (p *ParserImpl) parseShellInstruction() (*ShellInstruction, error) {
	startToken := p.advance()
	
	instr := &ShellInstruction{
		Location: &SourceLocation{
			Line:   startToken.Line,
			Column: startToken.Column,
		},
	}
	
	// Parse shell command (JSON array format)
	commands, _, err := p.parseCommand()
	if err != nil {
		return nil, err
	}
	
	instr.Shell = commands
	
	return instr, nil
}

// Helper methods for flag parsing...

func (p *ParserImpl) parseFlag(flagStr string, instr interface{}) error {
	// Remove -- prefix
	flagStr = strings.TrimPrefix(flagStr, "--")
	
	// Parse flag=value format
	parts := strings.SplitN(flagStr, "=", 2)
	flagName := parts[0]
	var flagValue string
	if len(parts) > 1 {
		flagValue = parts[1]
	}
	
	// Handle flag based on instruction type
	switch i := instr.(type) {
	case *FromInstruction:
		switch flagName {
		case "platform":
			i.Platform = flagValue
		default:
			return fmt.Errorf("unknown flag for FROM instruction: %s", flagName)
		}
	}
	
	return nil
}

// isValidRunFlag checks if a flag is a valid RUN instruction flag
func isValidRunFlag(flagStr string) bool {
	flagStr = strings.TrimPrefix(flagStr, "--")
	parts := strings.SplitN(flagStr, "=", 2)
	flagName := parts[0]
	
	switch flagName {
	case "mount", "network", "security":
		return true
	default:
		return false
	}
}

func (p *ParserImpl) parseRunFlag(flagStr string, instr *RunInstruction) error {
	flagStr = strings.TrimPrefix(flagStr, "--")
	parts := strings.SplitN(flagStr, "=", 2)
	flagName := parts[0]
	var flagValue string
	if len(parts) > 1 {
		flagValue = parts[1]
	}
	
	switch flagName {
	case "mount":
		mount, err := p.parseMountFlag(flagValue)
		if err != nil {
			return err
		}
		instr.Mounts = append(instr.Mounts, mount)
	case "network":
		instr.Network = flagValue
	case "security":
		instr.Security = flagValue
	default:
		return fmt.Errorf("unknown flag for RUN instruction: %s", flagName)
	}
	
	return nil
}

func (p *ParserImpl) parseCopyFlag(flagStr string, instr *CopyInstruction) error {
	flagStr = strings.TrimPrefix(flagStr, "--")
	parts := strings.SplitN(flagStr, "=", 2)
	flagName := parts[0]
	var flagValue string
	if len(parts) > 1 {
		flagValue = parts[1]
	}
	
	switch flagName {
	case "from":
		instr.From = flagValue
	case "chown":
		instr.Chown = flagValue
	case "chmod":
		instr.Chmod = flagValue
	default:
		return fmt.Errorf("unknown flag for COPY instruction: %s", flagName)
	}
	
	return nil
}

func (p *ParserImpl) parseAddFlag(flagStr string, instr *AddInstruction) error {
	flagStr = strings.TrimPrefix(flagStr, "--")
	parts := strings.SplitN(flagStr, "=", 2)
	flagName := parts[0]
	var flagValue string
	if len(parts) > 1 {
		flagValue = parts[1]
	}
	
	switch flagName {
	case "chown":
		instr.Chown = flagValue
	case "chmod":
		instr.Chmod = flagValue
	case "checksum":
		instr.Checksum = flagValue
	default:
		return fmt.Errorf("unknown flag for ADD instruction: %s", flagName)
	}
	
	return nil
}

func (p *ParserImpl) parseHealthcheckFlag(flagStr string, instr *HealthcheckInstruction) error {
	flagStr = strings.TrimPrefix(flagStr, "--")
	parts := strings.SplitN(flagStr, "=", 2)
	flagName := parts[0]
	var flagValue string
	if len(parts) > 1 {
		flagValue = parts[1]
	}
	
	switch flagName {
	case "interval":
		instr.Interval = flagValue
	case "timeout":
		instr.Timeout = flagValue
	case "start-period":
		instr.StartPeriod = flagValue
	case "retries":
		retries, err := strconv.Atoi(flagValue)
		if err != nil {
			return fmt.Errorf("invalid retries value: %s", flagValue)
		}
		instr.Retries = retries
	default:
		return fmt.Errorf("unknown flag for HEALTHCHECK instruction: %s", flagName)
	}
	
	return nil
}

func (p *ParserImpl) parseMountFlag(flagValue string) (*MountInstruction, error) {
	mount := &MountInstruction{
		Options: make(map[string]string),
	}
	
	// Parse mount options: type=cache,target=/cache,source=cache
	opts := strings.Split(flagValue, ",")
	for _, opt := range opts {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := parts[0]
		value := parts[1]
		
		switch key {
		case "type":
			mount.Type = value
		case "source":
			mount.Source = value
		case "target":
			mount.Target = value
		default:
			mount.Options[key] = value
		}
	}
	
	return mount, nil
}

// Helper methods for image reference parsing...

func (p *ParserImpl) parseImageReference(imageRef string, instr *FromInstruction) error {
	// Handle stage references first
	if _, exists := p.stageNames[imageRef]; exists {
		instr.Stage = imageRef
		return nil
	}
	
	// Parse registry/namespace/repository:tag@digest
	parts := strings.Split(imageRef, "@")
	var imageWithTag, digest string
	
	if len(parts) == 2 {
		imageWithTag = parts[0]
		digest = parts[1]
		instr.Digest = digest
	} else {
		imageWithTag = imageRef
	}
	
	// Parse tag
	tagParts := strings.Split(imageWithTag, ":")
	var imageName, tag string
	
	if len(tagParts) > 1 {
		// Check if the last part is a tag or port (for registry)
		lastPart := tagParts[len(tagParts)-1]
		if !strings.Contains(lastPart, "/") && !strings.Contains(lastPart, ".") {
			// It's a tag
			imageName = strings.Join(tagParts[:len(tagParts)-1], ":")
			tag = lastPart
			instr.Tag = tag
		} else {
			imageName = imageWithTag
		}
	} else {
		imageName = imageWithTag
	}
	
	instr.Image = imageName
	
	return nil
}

// Helper methods for navigation...

func (p *ParserImpl) peek() *Token {
	if p.current >= len(p.tokens) {
		return &Token{Type: TokenEOF}
	}
	return p.tokens[p.current]
}

func (p *ParserImpl) advance() *Token {
	if !p.isAtEnd() {
		p.current++
	}
	return p.previous()
}

func (p *ParserImpl) previous() *Token {
	if p.current > 0 {
		return p.tokens[p.current-1]
	}
	return &Token{Type: TokenEOF}
}

func (p *ParserImpl) isAtEnd() bool {
	return p.current >= len(p.tokens) || (p.current < len(p.tokens) && p.tokens[p.current].Type == TokenEOF)
}

// expandBuildArgs expands build arguments in a string.
func (p *ParserImpl) expandBuildArgs(value string) string {
	return expandArgs(value, p.buildArgs)
}

// getCurrentAST returns the current AST being built (placeholder).
func (p *ParserImpl) getCurrentAST() *AST {
	// This would need to be properly implemented to track current AST
	return nil
}

// validateStage validates a single stage.
func (p *ParserImpl) validateStage(stage *Stage, index int) error {
	if stage.From == nil {
		return fmt.Errorf("stage must have a FROM instruction")
	}
	
	// Validate FROM instruction
	if err := stage.From.Validate(); err != nil {
		return fmt.Errorf("FROM instruction validation failed: %w", err)
	}
	
	// Validate all instructions in the stage
	for i, instr := range stage.Instructions {
		if err := instr.Validate(); err != nil {
			return fmt.Errorf("instruction %d validation failed: %w", i, err)
		}
	}
	
	return nil
}

