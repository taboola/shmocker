// Package dockerfile provides validation functionality for Dockerfile AST.
package dockerfile

import (
	"fmt"
	"regexp"
	"strings"
)

// Validator provides comprehensive validation for Dockerfile AST.
type Validator struct {
	// Configuration options
	allowEmptyImages   bool
	strictFlagOrder    bool
	requireHealthcheck bool
}

// NewValidator creates a new validator with default settings.
func NewValidator() *Validator {
	return &Validator{
		allowEmptyImages:   false,
		strictFlagOrder:    true,
		requireHealthcheck: false,
	}
}

// ValidateAST performs comprehensive validation on a Dockerfile AST.
func (v *Validator) ValidateAST(ast *AST) error {
	if ast == nil {
		return fmt.Errorf("AST is nil")
	}
	
	if len(ast.Stages) == 0 {
		return fmt.Errorf("Dockerfile must contain at least one FROM instruction")
	}
	
	// Validate stage name uniqueness
	if err := v.validateStageNameUniqueness(ast.Stages); err != nil {
		return err
	}
	
	// Validate each stage
	for i, stage := range ast.Stages {
		if err := v.validateStage(stage, i, ast); err != nil {
			return fmt.Errorf("stage %d validation failed: %w", i, err)
		}
	}
	
	// Validate instruction order across stages
	if err := v.validateInstructionOrder(ast); err != nil {
		return err
	}
	
	// Validate cross-stage references
	if err := v.validateCrossStageReferences(ast); err != nil {
		return err
	}
	
	return nil
}

// validateStageNameUniqueness ensures all stage names are unique.
func (v *Validator) validateStageNameUniqueness(stages []*Stage) error {
	stageNames := make(map[string]int)
	
	for i, stage := range stages {
		if stage.Name != "" {
			if prevIndex, exists := stageNames[stage.Name]; exists {
				return fmt.Errorf("duplicate stage name '%s' at stage %d (previously defined at stage %d)", 
					stage.Name, i, prevIndex)
			}
			stageNames[stage.Name] = i
		}
	}
	
	return nil
}

// validateStage validates a single build stage.
func (v *Validator) validateStage(stage *Stage, index int, ast *AST) error {
	if stage.From == nil {
		return fmt.Errorf("stage must have a FROM instruction")
	}
	
	// Validate FROM instruction
	if err := v.validateFromInstruction(stage.From, index, ast); err != nil {
		return fmt.Errorf("FROM instruction validation failed: %w", err)
	}
	
	// Track instruction types for validation
	instructionTypes := make(map[string]int)
	
	// Validate all instructions in the stage
	for i, instr := range stage.Instructions {
		if err := v.validateInstruction(instr, i, stage, ast); err != nil {
			return fmt.Errorf("instruction %d (%s) validation failed: %w", i, instr.GetCmd(), err)
		}
		
		// Track instruction occurrences
		cmd := instr.GetCmd()
		instructionTypes[cmd]++
	}
	
	// Validate instruction occurrence limits
	if err := v.validateInstructionOccurrences(instructionTypes, stage); err != nil {
		return err
	}
	
	return nil
}

// validateFromInstruction validates a FROM instruction.
func (v *Validator) validateFromInstruction(from *FromInstruction, stageIndex int, ast *AST) error {
	// Basic validation
	if err := from.Validate(); err != nil {
		return err
	}
	
	// Validate stage reference
	if from.Stage != "" {
		// Must reference a previous stage
		stageFound := false
		for i, stage := range ast.Stages {
			if i >= stageIndex {
				break // Only check previous stages
			}
			if stage.Name == from.Stage {
				stageFound = true
				break
			}
		}
		if !stageFound {
			return fmt.Errorf("FROM references unknown stage '%s'", from.Stage)
		}
	} else if from.Image == "" && !v.allowEmptyImages {
		return fmt.Errorf("FROM instruction requires an image or stage reference")
	}
	
	// Validate image reference format
	if from.Image != "" {
		if err := v.validateImageReference(from.Image, from.Tag, from.Digest); err != nil {
			return fmt.Errorf("invalid image reference: %w", err)
		}
	}
	
	// Validate platform format
	if from.Platform != "" {
		if err := v.validatePlatform(from.Platform); err != nil {
			return fmt.Errorf("invalid platform specification: %w", err)
		}
	}
	
	return nil
}

// validateInstruction validates a single instruction.
func (v *Validator) validateInstruction(instr Instruction, index int, stage *Stage, ast *AST) error {
	// Basic instruction validation
	if err := instr.Validate(); err != nil {
		return err
	}
	
	// Type-specific validation
	switch i := instr.(type) {
	case *RunInstruction:
		return v.validateRunInstruction(i)
	case *CopyInstruction:
		return v.validateCopyInstruction(i, stage, ast)
	case *AddInstruction:
		return v.validateAddInstruction(i)
	case *EnvInstruction:
		return v.validateEnvInstruction(i)
	case *ExposeInstruction:
		return v.validateExposeInstruction(i)
	case *VolumeInstruction:
		return v.validateVolumeInstruction(i)
	case *UserInstruction:
		return v.validateUserInstruction(i)
	case *WorkdirInstruction:
		return v.validateWorkdirInstruction(i)
	case *LabelInstruction:
		return v.validateLabelInstruction(i)
	case *ArgInstruction:
		return v.validateArgInstruction(i)
	case *OnbuildInstruction:
		return v.validateOnbuildInstruction(i)
	case *HealthcheckInstruction:
		return v.validateHealthcheckInstruction(i)
	case *ShellInstruction:
		return v.validateShellInstruction(i)
	case *CmdInstruction:
		return v.validateCmdInstruction(i)
	case *EntrypointInstruction:
		return v.validateEntrypointInstruction(i)
	case *StopsignalInstruction:
		return v.validateStopsignalInstruction(i)
	}
	
	return nil
}

// validateRunInstruction validates a RUN instruction.
func (v *Validator) validateRunInstruction(run *RunInstruction) error {
	if len(run.Commands) == 0 {
		return fmt.Errorf("RUN instruction requires at least one command")
	}
	
	// Validate mount instructions
	for i, mount := range run.Mounts {
		if err := v.validateMountInstruction(mount); err != nil {
			return fmt.Errorf("mount %d validation failed: %w", i, err)
		}
	}
	
	// Validate network mode
	if run.Network != "" {
		validNetworks := []string{"default", "none", "host"}
		if !contains(validNetworks, run.Network) {
			return fmt.Errorf("invalid network mode '%s', must be one of: %s", 
				run.Network, strings.Join(validNetworks, ", "))
		}
	}
	
	// Validate security mode
	if run.Security != "" {
		validSecurity := []string{"insecure", "sandbox"}
		if !contains(validSecurity, run.Security) {
			return fmt.Errorf("invalid security mode '%s', must be one of: %s", 
				run.Security, strings.Join(validSecurity, ", "))
		}
	}
	
	return nil
}

// validateMountInstruction validates a mount instruction.
func (v *Validator) validateMountInstruction(mount *MountInstruction) error {
	if mount.Type == "" {
		return fmt.Errorf("mount type is required")
	}
	
	validTypes := []string{"bind", "cache", "tmpfs", "secret", "ssh"}
	if !contains(validTypes, mount.Type) {
		return fmt.Errorf("invalid mount type '%s', must be one of: %s", 
			mount.Type, strings.Join(validTypes, ", "))
	}
	
	if mount.Target == "" {
		return fmt.Errorf("mount target is required")
	}
	
	// Type-specific validation
	switch mount.Type {
	case "bind":
		if mount.Source == "" {
			return fmt.Errorf("bind mount requires source")
		}
	case "cache":
		// Cache mount validation - target is required (already checked)
	case "tmpfs":
		// tmpfs mount validation - only target is required
	case "secret":
		// Secret mount validation
		if mount.Source == "" {
			return fmt.Errorf("secret mount requires source (secret ID)")
		}
	case "ssh":
		// SSH mount validation - target is required (already checked)
	}
	
	return nil
}

// validateCopyInstruction validates a COPY instruction.
func (v *Validator) validateCopyInstruction(copy *CopyInstruction, stage *Stage, ast *AST) error {
	if len(copy.Sources) == 0 {
		return fmt.Errorf("COPY instruction requires at least one source")
	}
	
	if copy.Destination == "" {
		return fmt.Errorf("COPY instruction requires a destination")
	}
	
	// Validate from stage reference
	if copy.From != "" {
		if err := v.validateStageReference(copy.From, stage, ast); err != nil {
			return fmt.Errorf("invalid from stage reference: %w", err)
		}
	}
	
	// Validate chown format
	if copy.Chown != "" {
		if err := v.validateChownFormat(copy.Chown); err != nil {
			return fmt.Errorf("invalid chown format: %w", err)
		}
	}
	
	// Validate chmod format
	if copy.Chmod != "" {
		if err := v.validateChmodFormat(copy.Chmod); err != nil {
			return fmt.Errorf("invalid chmod format: %w", err)
		}
	}
	
	return nil
}

// validateAddInstruction validates an ADD instruction.
func (v *Validator) validateAddInstruction(add *AddInstruction) error {
	if len(add.Sources) == 0 {
		return fmt.Errorf("ADD instruction requires at least one source")
	}
	
	if add.Destination == "" {
		return fmt.Errorf("ADD instruction requires a destination")
	}
	
	// Validate chown format
	if add.Chown != "" {
		if err := v.validateChownFormat(add.Chown); err != nil {
			return fmt.Errorf("invalid chown format: %w", err)
		}
	}
	
	// Validate chmod format
	if add.Chmod != "" {
		if err := v.validateChmodFormat(add.Chmod); err != nil {
			return fmt.Errorf("invalid chmod format: %w", err)
		}
	}
	
	// Validate checksum format
	if add.Checksum != "" {
		if err := v.validateChecksumFormat(add.Checksum); err != nil {
			return fmt.Errorf("invalid checksum format: %w", err)
		}
	}
	
	return nil
}

// validateEnvInstruction validates an ENV instruction.
func (v *Validator) validateEnvInstruction(env *EnvInstruction) error {
	if len(env.Variables) == 0 {
		return fmt.Errorf("ENV instruction requires at least one variable")
	}
	
	// Validate variable names
	for name := range env.Variables {
		if err := v.validateEnvironmentVariableName(name); err != nil {
			return fmt.Errorf("invalid environment variable name '%s': %w", name, err)
		}
	}
	
	return nil
}

// validateExposeInstruction validates an EXPOSE instruction.
func (v *Validator) validateExposeInstruction(expose *ExposeInstruction) error {
	if len(expose.Ports) == 0 {
		return fmt.Errorf("EXPOSE instruction requires at least one port")
	}
	
	// Validate port formats
	for _, port := range expose.Ports {
		if err := validatePort(port); err != nil {
			return fmt.Errorf("invalid port format '%s': %w", port, err)
		}
	}
	
	return nil
}

// validateVolumeInstruction validates a VOLUME instruction.
func (v *Validator) validateVolumeInstruction(volume *VolumeInstruction) error {
	if len(volume.Paths) == 0 {
		return fmt.Errorf("VOLUME instruction requires at least one path")
	}
	
	// Validate paths are absolute
	for _, path := range volume.Paths {
		if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "$") {
			return fmt.Errorf("volume path '%s' should be absolute", path)
		}
	}
	
	return nil
}

// validateUserInstruction validates a USER instruction.
func (v *Validator) validateUserInstruction(user *UserInstruction) error {
	if user.User == "" {
		return fmt.Errorf("USER instruction requires a user")
	}
	
	// Validate user format (can be name or UID)
	if err := v.validateUserFormat(user.User); err != nil {
		return fmt.Errorf("invalid user format: %w", err)
	}
	
	// Validate group format if specified
	if user.Group != "" {
		if err := v.validateUserFormat(user.Group); err != nil {
			return fmt.Errorf("invalid group format: %w", err)
		}
	}
	
	return nil
}

// validateWorkdirInstruction validates a WORKDIR instruction.
func (v *Validator) validateWorkdirInstruction(workdir *WorkdirInstruction) error {
	if workdir.Path == "" {
		return fmt.Errorf("WORKDIR instruction requires a path")
	}
	
	// Path validation - can be relative or absolute
	if strings.Contains(workdir.Path, "..") {
		return fmt.Errorf("WORKDIR path should not contain '..' for security reasons")
	}
	
	return nil
}

// validateLabelInstruction validates a LABEL instruction.
func (v *Validator) validateLabelInstruction(label *LabelInstruction) error {
	if len(label.Labels) == 0 {
		return fmt.Errorf("LABEL instruction requires at least one label")
	}
	
	// Validate label keys according to OCI spec
	for key := range label.Labels {
		if err := v.validateLabelKey(key); err != nil {
			return fmt.Errorf("invalid label key '%s': %w", key, err)
		}
	}
	
	return nil
}

// validateArgInstruction validates an ARG instruction.
func (v *Validator) validateArgInstruction(arg *ArgInstruction) error {
	if arg.Name == "" {
		return fmt.Errorf("ARG instruction requires a name")
	}
	
	// Validate ARG name format
	if err := v.validateEnvironmentVariableName(arg.Name); err != nil {
		return fmt.Errorf("invalid ARG name '%s': %w", arg.Name, err)
	}
	
	return nil
}

// validateOnbuildInstruction validates an ONBUILD instruction.
func (v *Validator) validateOnbuildInstruction(onbuild *OnbuildInstruction) error {
	if onbuild.Instruction == nil {
		return fmt.Errorf("ONBUILD instruction requires a sub-instruction")
	}
	
	// ONBUILD cannot contain certain instructions
	cmd := onbuild.Instruction.GetCmd()
	forbiddenInstructions := []string{"FROM", "ONBUILD", "MAINTAINER"}
	
	if contains(forbiddenInstructions, cmd) {
		return fmt.Errorf("ONBUILD instruction cannot contain %s", cmd)
	}
	
	// Recursively validate the sub-instruction
	return onbuild.Instruction.Validate()
}

// validateHealthcheckInstruction validates a HEALTHCHECK instruction.
func (v *Validator) validateHealthcheckInstruction(health *HealthcheckInstruction) error {
	if health.Type != "NONE" && health.Type != "CMD" {
		return fmt.Errorf("HEALTHCHECK type must be NONE or CMD")
	}
	
	if health.Type == "CMD" && len(health.Commands) == 0 {
		return fmt.Errorf("HEALTHCHECK CMD requires at least one command")
	}
	
	// Validate duration formats
	if health.Interval != "" {
		if err := v.validateDuration(health.Interval); err != nil {
			return fmt.Errorf("invalid interval duration: %w", err)
		}
	}
	
	if health.Timeout != "" {
		if err := v.validateDuration(health.Timeout); err != nil {
			return fmt.Errorf("invalid timeout duration: %w", err)
		}
	}
	
	if health.StartPeriod != "" {
		if err := v.validateDuration(health.StartPeriod); err != nil {
			return fmt.Errorf("invalid start-period duration: %w", err)
		}
	}
	
	if health.Retries < 0 {
		return fmt.Errorf("retries must be non-negative")
	}
	
	return nil
}

// validateShellInstruction validates a SHELL instruction.
func (v *Validator) validateShellInstruction(shell *ShellInstruction) error {
	if len(shell.Shell) == 0 {
		return fmt.Errorf("SHELL instruction requires at least one argument")
	}
	
	// First argument should be the shell executable
	if shell.Shell[0] == "" {
		return fmt.Errorf("SHELL executable cannot be empty")
	}
	
	return nil
}

// validateCmdInstruction validates a CMD instruction.
func (v *Validator) validateCmdInstruction(cmd *CmdInstruction) error {
	if len(cmd.Commands) == 0 {
		return fmt.Errorf("CMD instruction requires at least one command")
	}
	
	return nil
}

// validateEntrypointInstruction validates an ENTRYPOINT instruction.
func (v *Validator) validateEntrypointInstruction(entrypoint *EntrypointInstruction) error {
	if len(entrypoint.Commands) == 0 {
		return fmt.Errorf("ENTRYPOINT instruction requires at least one command")
	}
	
	return nil
}

// validateStopsignalInstruction validates a STOPSIGNAL instruction.
func (v *Validator) validateStopsignalInstruction(stopsignal *StopsignalInstruction) error {
	if stopsignal.Signal == "" {
		return fmt.Errorf("STOPSIGNAL instruction requires a signal")
	}
	
	// Validate signal format (number or name)
	if err := v.validateSignalFormat(stopsignal.Signal); err != nil {
		return fmt.Errorf("invalid signal format: %w", err)
	}
	
	return nil
}

// validateInstructionOrder validates instruction ordering rules.
func (v *Validator) validateInstructionOrder(ast *AST) error {
	// ARG instructions before FROM are allowed
	// Each stage must start with FROM
	// Some instructions have ordering dependencies
	
	for stageIndex, stage := range ast.Stages {
		// Validate instruction order within stage
		if err := v.validateStageInstructionOrder(stage, stageIndex); err != nil {
			return fmt.Errorf("instruction order validation failed for stage %d: %w", stageIndex, err)
		}
	}
	
	return nil
}

// validateStageInstructionOrder validates instruction order within a stage.
func (v *Validator) validateStageInstructionOrder(stage *Stage, stageIndex int) error {
	// Track seen instructions for order validation
	seenHealthcheck := false
	
	for i, instr := range stage.Instructions {
		cmd := instr.GetCmd()
		
		switch cmd {
		case "HEALTHCHECK":
			if seenHealthcheck {
				return fmt.Errorf("only one HEALTHCHECK instruction is allowed per stage")
			}
			seenHealthcheck = true
		}
		
		// Additional order checks can be added here
		_ = i // Suppress unused variable warning
		_ = instr // Suppress unused variable warning
	}
	
	return nil
}

// validateInstructionOccurrences validates instruction occurrence limits.
func (v *Validator) validateInstructionOccurrences(counts map[string]int, stage *Stage) error {
	// Instructions that can only appear once per stage
	singleOccurrence := []string{"CMD", "ENTRYPOINT", "HEALTHCHECK"}
	
	for _, cmd := range singleOccurrence {
		if count, exists := counts[cmd]; exists && count > 1 {
			return fmt.Errorf("instruction %s can only appear once per stage, found %d occurrences", cmd, count)
		}
	}
	
	return nil
}

// validateCrossStageReferences validates references between stages.
func (v *Validator) validateCrossStageReferences(ast *AST) error {
	// Build map of stage names to indices
	stageNames := make(map[string]int)
	for i, stage := range ast.Stages {
		if stage.Name != "" {
			stageNames[stage.Name] = i
		}
	}
	
	// Validate COPY --from references
	for stageIndex, stage := range ast.Stages {
		for _, instr := range stage.Instructions {
			if copy, ok := instr.(*CopyInstruction); ok && copy.From != "" {
				if err := v.validateStageReference(copy.From, stage, ast); err != nil {
					return fmt.Errorf("stage %d COPY --from validation failed: %w", stageIndex, err)
				}
			}
		}
	}
	
	return nil
}

// Helper validation functions...

// validateImageReference validates container image reference format.
func (v *Validator) validateImageReference(image, tag, digest string) error {
	// Basic image name validation
	if image == "" {
		return fmt.Errorf("image name cannot be empty")
	}
	
	// Validate image name format
	imagePattern := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	if !imagePattern.MatchString(strings.ToLower(image)) {
		return fmt.Errorf("invalid image name format")
	}
	
	// Validate tag format
	if tag != "" {
		tagPattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
		if !tagPattern.MatchString(tag) || len(tag) > 128 {
			return fmt.Errorf("invalid tag format")
		}
	}
	
	// Validate digest format
	if digest != "" {
		digestPattern := regexp.MustCompile(`^[a-z0-9]+:[a-f0-9]{64}$`)
		if !digestPattern.MatchString(digest) {
			return fmt.Errorf("invalid digest format")
		}
	}
	
	return nil
}

// validatePlatform validates platform specification format.
func (v *Validator) validatePlatform(platform string) error {
	// Platform format: os[/arch[/variant]]
	parts := strings.Split(platform, "/")
	if len(parts) == 0 || len(parts) > 3 {
		return fmt.Errorf("invalid platform format, expected os[/arch[/variant]]")
	}
	
	// Validate OS
	validOS := []string{"linux", "windows", "darwin", "freebsd"}
	if !contains(validOS, parts[0]) {
		return fmt.Errorf("unsupported OS '%s'", parts[0])
	}
	
	// Validate architecture if specified
	if len(parts) > 1 {
		validArch := []string{"amd64", "arm64", "arm", "386", "ppc64le", "s390x"}
		if !contains(validArch, parts[1]) {
			return fmt.Errorf("unsupported architecture '%s'", parts[1])
		}
	}
	
	return nil
}

// validateStageReference validates a stage reference (for COPY --from).
func (v *Validator) validateStageReference(ref string, currentStage *Stage, ast *AST) error {
	// Can reference previous stage by name or index
	
	// Try to parse as stage index
	if stageIndex := parseStageIndex(ref); stageIndex >= 0 {
		if stageIndex >= len(ast.Stages) {
			return fmt.Errorf("stage index %d is out of range", stageIndex)
		}
		// Must reference previous stage
		currentIndex := -1
		for i, stage := range ast.Stages {
			if stage == currentStage {
				currentIndex = i
				break
			}
		}
		if stageIndex >= currentIndex {
			return fmt.Errorf("cannot reference current or future stage")
		}
		return nil
	}
	
	// Try to match stage name
	for _, stage := range ast.Stages {
		if stage == currentStage {
			break // Only check previous stages
		}
		if stage.Name == ref {
			return nil
		}
	}
	
	return fmt.Errorf("unknown stage reference '%s'", ref)
}

// validateChownFormat validates chown specification format.
func (v *Validator) validateChownFormat(chown string) error {
	// Format: user[:group] where user/group can be name or UID/GID
	parts := strings.Split(chown, ":")
	if len(parts) > 2 {
		return fmt.Errorf("invalid chown format, expected user[:group]")
	}
	
	// Validate user
	if err := v.validateUserFormat(parts[0]); err != nil {
		return fmt.Errorf("invalid user in chown: %w", err)
	}
	
	// Validate group if specified
	if len(parts) == 2 && parts[1] != "" {
		if err := v.validateUserFormat(parts[1]); err != nil {
			return fmt.Errorf("invalid group in chown: %w", err)
		}
	}
	
	return nil
}

// validateChmodFormat validates chmod specification format.
func (v *Validator) validateChmodFormat(chmod string) error {
	// Validate octal format (e.g., 755, 0644)
	chmodPattern := regexp.MustCompile(`^0?[0-7]{3,4}$`)
	if !chmodPattern.MatchString(chmod) {
		return fmt.Errorf("chmod must be in octal format (e.g., 755, 0644)")
	}
	
	return nil
}

// validateChecksumFormat validates checksum specification format.
func (v *Validator) validateChecksumFormat(checksum string) error {
	// Format: algorithm:hash
	parts := strings.Split(checksum, ":")
	if len(parts) != 2 {
		return fmt.Errorf("checksum format must be algorithm:hash")
	}
	
	algorithm := parts[0]
	hash := parts[1]
	
	// Validate algorithm
	validAlgorithms := []string{"md5", "sha1", "sha256", "sha512"}
	if !contains(validAlgorithms, algorithm) {
		return fmt.Errorf("unsupported checksum algorithm '%s'", algorithm)
	}
	
	// Validate hash format
	expectedLength := map[string]int{
		"md5":    32,
		"sha1":   40,
		"sha256": 64,
		"sha512": 128,
	}
	
	if len(hash) != expectedLength[algorithm] {
		return fmt.Errorf("invalid %s hash length, expected %d characters", algorithm, expectedLength[algorithm])
	}
	
	hashPattern := regexp.MustCompile(`^[a-f0-9]+$`)
	if !hashPattern.MatchString(hash) {
		return fmt.Errorf("hash must contain only hexadecimal characters")
	}
	
	return nil
}

// validateEnvironmentVariableName validates environment variable name format.
func (v *Validator) validateEnvironmentVariableName(name string) error {
	if name == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}
	
	// Environment variable names must start with letter or underscore
	// and contain only letters, digits, and underscores
	envPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !envPattern.MatchString(name) {
		return fmt.Errorf("invalid environment variable name format")
	}
	
	return nil
}

// validateUserFormat validates user/group format (name or numeric ID).
func (v *Validator) validateUserFormat(user string) error {
	if user == "" {
		return fmt.Errorf("user cannot be empty")
	}
	
	// Can be numeric ID or name
	numericPattern := regexp.MustCompile(`^\d+$`)
	namePattern := regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	
	if !numericPattern.MatchString(user) && !namePattern.MatchString(user) {
		return fmt.Errorf("user must be numeric ID or valid username")
	}
	
	return nil
}

// validateLabelKey validates OCI label key format.
func (v *Validator) validateLabelKey(key string) error {
	if key == "" {
		return fmt.Errorf("label key cannot be empty")
	}
	
	// OCI label keys should follow reverse DNS notation for namespaced keys
	// or be simple names for non-namespaced keys
	if strings.Contains(key, ".") {
		// Reverse DNS format validation
		dnsPattern := regexp.MustCompile(`^[a-z0-9]+([.-][a-z0-9]+)*$`)
		if !dnsPattern.MatchString(key) {
			return fmt.Errorf("namespaced label key must follow reverse DNS notation")
		}
	} else {
		// Simple name validation
		namePattern := regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
		if !namePattern.MatchString(key) {
			return fmt.Errorf("invalid label key format")
		}
	}
	
	return nil
}

// validateDuration validates duration format for HEALTHCHECK.
func (v *Validator) validateDuration(duration string) error {
	// Duration format: number followed by unit (s, m, h)
	durationPattern := regexp.MustCompile(`^\d+[smh]$`)
	if !durationPattern.MatchString(duration) {
		return fmt.Errorf("duration must be number followed by s, m, or h")
	}
	
	return nil
}

// validateSignalFormat validates signal format for STOPSIGNAL.
func (v *Validator) validateSignalFormat(signal string) error {
	// Can be signal name (SIGTERM) or number (15)
	numericPattern := regexp.MustCompile(`^\d+$`)
	namePattern := regexp.MustCompile(`^SIG[A-Z]+$`)
	
	if !numericPattern.MatchString(signal) && !namePattern.MatchString(signal) {
		return fmt.Errorf("signal must be numeric or SIG* format")
	}
	
	return nil
}

// Helper functions...

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// parseStageIndex attempts to parse a stage reference as numeric index.
func parseStageIndex(ref string) int {
	// Simple numeric parsing - returns -1 if not numeric
	if len(ref) == 0 {
		return -1
	}
	
	result := 0
	for _, r := range ref {
		if r < '0' || r > '9' {
			return -1
		}
		result = result*10 + int(r-'0')
	}
	
	return result
}