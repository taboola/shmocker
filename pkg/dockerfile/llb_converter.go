// Package dockerfile provides LLB conversion functionality for Dockerfile AST.
package dockerfile

import (
	"fmt"
	"strings"
)

// LLBConverterImpl implements the LLBConverter interface.
type LLBConverterImpl struct {
	// Configuration
	buildArgs   map[string]string
	platform    string
	targetStage string
	labels      map[string]string
}

// NewLLBConverter creates a new LLB converter.
func NewLLBConverter() LLBConverter {
	return &LLBConverterImpl{
		buildArgs: make(map[string]string),
		labels:    make(map[string]string),
	}
}

// Convert transforms a Dockerfile AST into LLB definition.
func (c *LLBConverterImpl) Convert(ast *AST, opts *ConvertOptions) (*LLBDefinition, error) {
	if ast == nil {
		return nil, fmt.Errorf("AST is nil")
	}
	
	if len(ast.Stages) == 0 {
		return nil, fmt.Errorf("no stages found in AST")
	}
	
	// Apply options
	if opts != nil {
		if opts.BuildArgs != nil {
			c.buildArgs = opts.BuildArgs
		}
		if opts.Platform != "" {
			c.platform = opts.Platform
		}
		if opts.Target != "" {
			c.targetStage = opts.Target
		}
		if opts.Labels != nil {
			c.labels = opts.Labels
		}
	}
	
	// Determine target stage
	targetIndex := len(ast.Stages) - 1 // Default to last stage
	if c.targetStage != "" {
		found := false
		for i, stage := range ast.Stages {
			if stage.Name == c.targetStage {
				targetIndex = i
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("target stage '%s' not found", c.targetStage)
		}
	}
	
	// Convert stages up to target
	stageStates := make(map[int]*LLBState)
	stageNames := make(map[string]int)
	
	for i := 0; i <= targetIndex; i++ {
		stage := ast.Stages[i]
		
		// Track stage names
		if stage.Name != "" {
			stageNames[stage.Name] = i
		}
		
		// Convert stage
		state, err := c.ConvertStage(stage, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to convert stage %d: %w", i, err)
		}
		
		stageStates[i] = state
	}
	
	// Build final LLB definition
	finalState := stageStates[targetIndex]
	definition := &LLBDefinition{
		Definition: c.serializeState(finalState),
		Metadata:   c.buildMetadata(ast, opts),
	}
	
	return definition, nil
}

// ConvertStage converts a single build stage to LLB.
func (c *LLBConverterImpl) ConvertStage(stage *Stage, opts *ConvertOptions) (*LLBState, error) {
	if stage == nil {
		return nil, fmt.Errorf("stage is nil")
	}
	
	if stage.From == nil {
		return nil, fmt.Errorf("stage must have FROM instruction")
	}
	
	// Resolve base image
	baseImageRef, err := c.ResolveBaseImage(stage.From)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base image: %w", err)
	}
	
	// Create initial state from base image
	state := &LLBState{
		State: map[string]interface{}{
			"type":  "image",
			"image": baseImageRef.String(),
		},
		Metadata: make(map[string]interface{}),
	}
	
	// Set platform if specified
	if stage.Platform != "" || c.platform != "" {
		platform := stage.Platform
		if platform == "" {
			platform = c.platform
		}
		state.State.(map[string]interface{})["platform"] = platform
	}
	
	// Apply instructions
	for i, instr := range stage.Instructions {
		newState, err := c.convertInstruction(instr, state, stage, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to convert instruction %d (%s): %w", i, instr.GetCmd(), err)
		}
		state = newState
	}
	
	return state, nil
}

// ResolveBaseImage resolves the base image reference for a stage.
func (c *LLBConverterImpl) ResolveBaseImage(from *FromInstruction) (*ImageReference, error) {
	if from == nil {
		return nil, fmt.Errorf("FROM instruction is nil")
	}
	
	ref := &ImageReference{}
	
	// Handle stage reference
	if from.Stage != "" {
		// This is a reference to another stage
		// In a real implementation, this would be resolved to the actual stage output
		ref.Repository = from.Stage
		return ref, nil
	}
	
	// Parse image reference
	if from.Image == "" {
		return nil, fmt.Errorf("FROM instruction requires image or stage reference")
	}
	
	// Parse registry/namespace/repository
	imageParts := strings.Split(from.Image, "/")
	switch len(imageParts) {
	case 1:
		// Just repository name (e.g., "ubuntu")
		ref.Repository = imageParts[0]
	case 2:
		// namespace/repository (e.g., "library/ubuntu") or registry/repository
		if strings.Contains(imageParts[0], ".") || strings.Contains(imageParts[0], ":") {
			// Registry/repository
			ref.Registry = imageParts[0]
			ref.Repository = imageParts[1]
		} else {
			// Namespace/repository
			ref.Namespace = imageParts[0]
			ref.Repository = imageParts[1]
		}
	case 3:
		// registry/namespace/repository
		ref.Registry = imageParts[0]
		ref.Namespace = imageParts[1]
		ref.Repository = imageParts[2]
	default:
		// registry/namespace/repo/sub-repo/...
		ref.Registry = imageParts[0]
		ref.Namespace = imageParts[1]
		ref.Repository = strings.Join(imageParts[2:], "/")
	}
	
	// Set tag and digest
	if from.Tag != "" {
		ref.Tag = from.Tag
	} else {
		ref.Tag = "latest" // Default tag
	}
	
	if from.Digest != "" {
		ref.Digest = from.Digest
	}
	
	return ref, nil
}

// convertInstruction converts a single instruction to LLB operations.
func (c *LLBConverterImpl) convertInstruction(instr Instruction, currentState *LLBState, stage *Stage, opts *ConvertOptions) (*LLBState, error) {
	switch i := instr.(type) {
	case *RunInstruction:
		return c.convertRunInstruction(i, currentState, opts)
	case *CopyInstruction:
		return c.convertCopyInstruction(i, currentState, stage, opts)
	case *AddInstruction:
		return c.convertAddInstruction(i, currentState, opts)
	case *EnvInstruction:
		return c.convertEnvInstruction(i, currentState, opts)
	case *WorkdirInstruction:
		return c.convertWorkdirInstruction(i, currentState, opts)
	case *UserInstruction:
		return c.convertUserInstruction(i, currentState, opts)
	case *VolumeInstruction:
		return c.convertVolumeInstruction(i, currentState, opts)
	case *ExposeInstruction:
		return c.convertExposeInstruction(i, currentState, opts)
	case *LabelInstruction:
		return c.convertLabelInstruction(i, currentState, opts)
	case *ArgInstruction:
		return c.convertArgInstruction(i, currentState, opts)
	case *CmdInstruction:
		return c.convertCmdInstruction(i, currentState, opts)
	case *EntrypointInstruction:
		return c.convertEntrypointInstruction(i, currentState, opts)
	case *HealthcheckInstruction:
		return c.convertHealthcheckInstruction(i, currentState, opts)
	case *StopsignalInstruction:
		return c.convertStopsignalInstruction(i, currentState, opts)
	case *ShellInstruction:
		return c.convertShellInstruction(i, currentState, opts)
	case *OnbuildInstruction:
		// ONBUILD instructions are metadata and don't affect the build directly
		return c.convertOnbuildInstruction(i, currentState, opts)
	default:
		return nil, fmt.Errorf("unsupported instruction type: %T", instr)
	}
}

// convertRunInstruction converts a RUN instruction to LLB.
func (c *LLBConverterImpl) convertRunInstruction(run *RunInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	if len(run.Commands) == 0 {
		return nil, fmt.Errorf("RUN instruction has no commands")
	}
	
	// Create exec operation
	exec := map[string]interface{}{
		"type": "exec",
		"meta": map[string]interface{}{
			"args": run.Commands,
		},
	}
	
	// Set shell vs exec form
	if run.Shell {
		// Shell form - wrap in shell
		shell := []string{"/bin/sh", "-c"}
		command := strings.Join(run.Commands, " ")
		exec["meta"].(map[string]interface{})["args"] = append(shell, command)
	}
	
	// Add mounts
	if len(run.Mounts) > 0 {
		mounts := make([]map[string]interface{}, len(run.Mounts))
		for i, mount := range run.Mounts {
			mounts[i] = c.convertMountInstruction(mount)
		}
		exec["mounts"] = mounts
	}
	
	// Add network mode
	if run.Network != "" {
		exec["network"] = run.Network
	}
	
	// Add security mode
	if run.Security != "" {
		exec["security"] = run.Security
	}
	
	// Create new state
	newState := &LLBState{
		State: exec,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy metadata from current state
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	return newState, nil
}

// convertCopyInstruction converts a COPY instruction to LLB.
func (c *LLBConverterImpl) convertCopyInstruction(copy *CopyInstruction, currentState *LLBState, stage *Stage, opts *ConvertOptions) (*LLBState, error) {
	if len(copy.Sources) == 0 {
		return nil, fmt.Errorf("COPY instruction has no sources")
	}
	
	// Create file operation
	fileOp := map[string]interface{}{
		"type": "file",
		"actions": []map[string]interface{}{
			{
				"action": "copy",
				"src":    strings.Join(copy.Sources, " "),
				"dest":   copy.Destination,
			},
		},
	}
	
	// Handle --from flag
	if copy.From != "" {
		fileOp["from"] = copy.From
	}
	
	// Handle --chown flag
	if copy.Chown != "" {
		fileOp["actions"].([]map[string]interface{})[0]["chown"] = copy.Chown
	}
	
	// Handle --chmod flag
	if copy.Chmod != "" {
		fileOp["actions"].([]map[string]interface{})[0]["chmod"] = copy.Chmod
	}
	
	// Create new state
	newState := &LLBState{
		State: fileOp,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy metadata from current state
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	return newState, nil
}

// convertAddInstruction converts an ADD instruction to LLB.
func (c *LLBConverterImpl) convertAddInstruction(add *AddInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	if len(add.Sources) == 0 {
		return nil, fmt.Errorf("ADD instruction has no sources")
	}
	
	// ADD is similar to COPY but with additional features
	fileOp := map[string]interface{}{
		"type": "file",
		"actions": []map[string]interface{}{
			{
				"action": "add",
				"src":    strings.Join(add.Sources, " "),
				"dest":   add.Destination,
			},
		},
	}
	
	// Handle --chown flag
	if add.Chown != "" {
		fileOp["actions"].([]map[string]interface{})[0]["chown"] = add.Chown
	}
	
	// Handle --chmod flag
	if add.Chmod != "" {
		fileOp["actions"].([]map[string]interface{})[0]["chmod"] = add.Chmod
	}
	
	// Handle --checksum flag
	if add.Checksum != "" {
		fileOp["actions"].([]map[string]interface{})[0]["checksum"] = add.Checksum
	}
	
	// Create new state
	newState := &LLBState{
		State: fileOp,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy metadata from current state
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	return newState, nil
}

// convertEnvInstruction converts an ENV instruction to LLB.
func (c *LLBConverterImpl) convertEnvInstruction(env *EnvInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// ENV instructions set environment variables in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Add environment variables
	envVars := make(map[string]string)
	if existing, ok := newState.Metadata["env"].(map[string]string); ok {
		for k, v := range existing {
			envVars[k] = v
		}
	}
	
	for k, v := range env.Variables {
		envVars[k] = c.expandBuildArgs(v)
	}
	
	newState.Metadata["env"] = envVars
	
	return newState, nil
}

// convertWorkdirInstruction converts a WORKDIR instruction to LLB.
func (c *LLBConverterImpl) convertWorkdirInstruction(workdir *WorkdirInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// WORKDIR sets the working directory in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set working directory
	newState.Metadata["workdir"] = c.expandBuildArgs(workdir.Path)
	
	return newState, nil
}

// convertUserInstruction converts a USER instruction to LLB.
func (c *LLBConverterImpl) convertUserInstruction(user *UserInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// USER sets the user in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set user
	userSpec := c.expandBuildArgs(user.User)
	if user.Group != "" {
		userSpec += ":" + c.expandBuildArgs(user.Group)
	}
	newState.Metadata["user"] = userSpec
	
	return newState, nil
}

// convertVolumeInstruction converts a VOLUME instruction to LLB.
func (c *LLBConverterImpl) convertVolumeInstruction(volume *VolumeInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// VOLUME declares mount points in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Add volumes
	volumes := make([]string, 0)
	if existing, ok := newState.Metadata["volumes"].([]string); ok {
		volumes = existing
	}
	
	for _, path := range volume.Paths {
		volumes = append(volumes, c.expandBuildArgs(path))
	}
	
	newState.Metadata["volumes"] = volumes
	
	return newState, nil
}

// convertExposeInstruction converts an EXPOSE instruction to LLB.
func (c *LLBConverterImpl) convertExposeInstruction(expose *ExposeInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// EXPOSE declares ports in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Add exposed ports
	ports := make([]string, 0)
	if existing, ok := newState.Metadata["expose"].([]string); ok {
		ports = existing
	}
	
	for _, port := range expose.Ports {
		ports = append(ports, c.expandBuildArgs(port))
	}
	
	newState.Metadata["expose"] = ports
	
	return newState, nil
}

// convertLabelInstruction converts a LABEL instruction to LLB.
func (c *LLBConverterImpl) convertLabelInstruction(label *LabelInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// LABEL sets labels in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Add labels
	labels := make(map[string]string)
	if existing, ok := newState.Metadata["labels"].(map[string]string); ok {
		for k, v := range existing {
			labels[k] = v
		}
	}
	
	for k, v := range label.Labels {
		labels[k] = c.expandBuildArgs(v)
	}
	
	newState.Metadata["labels"] = labels
	
	return newState, nil
}

// convertArgInstruction converts an ARG instruction to LLB.
func (c *LLBConverterImpl) convertArgInstruction(arg *ArgInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// ARG instructions define build arguments
	// They don't change the state but may affect build arg resolution
	
	// Add to build args if not already set
	if _, exists := c.buildArgs[arg.Name]; !exists && arg.DefaultValue != "" {
		c.buildArgs[arg.Name] = arg.DefaultValue
	}
	
	// ARG doesn't change the state
	return currentState, nil
}

// convertCmdInstruction converts a CMD instruction to LLB.
func (c *LLBConverterImpl) convertCmdInstruction(cmd *CmdInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// CMD sets the default command in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set command
	cmdSpec := make(map[string]interface{})
	cmdSpec["args"] = cmd.Commands
	cmdSpec["shell"] = cmd.Shell
	
	newState.Metadata["cmd"] = cmdSpec
	
	return newState, nil
}

// convertEntrypointInstruction converts an ENTRYPOINT instruction to LLB.
func (c *LLBConverterImpl) convertEntrypointInstruction(entrypoint *EntrypointInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// ENTRYPOINT sets the entrypoint in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set entrypoint
	entrypointSpec := make(map[string]interface{})
	entrypointSpec["args"] = entrypoint.Commands
	entrypointSpec["shell"] = entrypoint.Shell
	
	newState.Metadata["entrypoint"] = entrypointSpec
	
	return newState, nil
}

// convertHealthcheckInstruction converts a HEALTHCHECK instruction to LLB.
func (c *LLBConverterImpl) convertHealthcheckInstruction(health *HealthcheckInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// HEALTHCHECK sets healthcheck configuration in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set healthcheck
	healthSpec := make(map[string]interface{})
	healthSpec["type"] = health.Type
	
	if health.Type == "CMD" {
		healthSpec["test"] = health.Commands
	}
	
	if health.Interval != "" {
		healthSpec["interval"] = health.Interval
	}
	if health.Timeout != "" {
		healthSpec["timeout"] = health.Timeout
	}
	if health.StartPeriod != "" {
		healthSpec["start_period"] = health.StartPeriod
	}
	if health.Retries > 0 {
		healthSpec["retries"] = health.Retries
	}
	
	newState.Metadata["healthcheck"] = healthSpec
	
	return newState, nil
}

// convertStopsignalInstruction converts a STOPSIGNAL instruction to LLB.
func (c *LLBConverterImpl) convertStopsignalInstruction(stopsignal *StopsignalInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// STOPSIGNAL sets the stop signal in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set stop signal
	newState.Metadata["stopsignal"] = c.expandBuildArgs(stopsignal.Signal)
	
	return newState, nil
}

// convertShellInstruction converts a SHELL instruction to LLB.
func (c *LLBConverterImpl) convertShellInstruction(shell *ShellInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// SHELL sets the default shell in the image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Set shell
	newState.Metadata["shell"] = shell.Shell
	
	return newState, nil
}

// convertOnbuildInstruction converts an ONBUILD instruction to LLB.
func (c *LLBConverterImpl) convertOnbuildInstruction(onbuild *OnbuildInstruction, currentState *LLBState, opts *ConvertOptions) (*LLBState, error) {
	// ONBUILD adds trigger instructions to image metadata
	newState := &LLBState{
		State: currentState.State,
		Metadata: make(map[string]interface{}),
	}
	
	// Copy existing metadata
	for k, v := range currentState.Metadata {
		newState.Metadata[k] = v
	}
	
	// Add ONBUILD instruction
	onbuilds := make([]string, 0)
	if existing, ok := newState.Metadata["onbuild"].([]string); ok {
		onbuilds = existing
	}
	
	onbuilds = append(onbuilds, onbuild.Instruction.String())
	newState.Metadata["onbuild"] = onbuilds
	
	return newState, nil
}

// convertMountInstruction converts a mount instruction to LLB mount.
func (c *LLBConverterImpl) convertMountInstruction(mount *MountInstruction) map[string]interface{} {
	mountSpec := map[string]interface{}{
		"type":   mount.Type,
		"target": mount.Target,
	}
	
	if mount.Source != "" {
		mountSpec["source"] = mount.Source
	}
	
	// Add options
	for k, v := range mount.Options {
		mountSpec[k] = v
	}
	
	return mountSpec
}

// serializeState serializes an LLB state to bytes.
func (c *LLBConverterImpl) serializeState(state *LLBState) []byte {
	// This is a placeholder - real implementation would use BuildKit's LLB serialization
	// For now, we'll create a simple JSON-like representation
	data := fmt.Sprintf(`{"state": %v, "metadata": %v}`, state.State, state.Metadata)
	return []byte(data)
}

// buildMetadata builds LLB metadata from AST and options.
func (c *LLBConverterImpl) buildMetadata(ast *AST, opts *ConvertOptions) map[string][]byte {
	metadata := make(map[string][]byte)
	
	// Add dockerfile metadata
	if ast.Metadata != nil {
		dockerfileData := fmt.Sprintf(`{"parser_version": "%s", "parse_time": "%s"}`, 
			ast.Metadata.ParserVersion, ast.Metadata.ParseTime.String())
		metadata["dockerfile"] = []byte(dockerfileData)
	}
	
	// Add build args metadata
	if len(c.buildArgs) > 0 {
		buildArgsData := "{"
		first := true
		for k, v := range c.buildArgs {
			if !first {
				buildArgsData += ", "
			}
			buildArgsData += fmt.Sprintf(`"%s": "%s"`, k, v)
			first = false
		}
		buildArgsData += "}"
		metadata["build_args"] = []byte(buildArgsData)
	}
	
	return metadata
}

// expandBuildArgs expands build arguments in a string.
func (c *LLBConverterImpl) expandBuildArgs(value string) string {
	return expandArgs(value, c.buildArgs)
}