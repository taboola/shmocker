// Package dockerfile provides instruction implementations for Dockerfile AST.
package dockerfile

import (
	"fmt"
	"strconv"
	"strings"
)

// CmdInstruction represents a CMD instruction.
type CmdInstruction struct {
	// Commands contains the command to execute
	Commands []string `json:"commands"`
	
	// Shell indicates if this is a shell form command
	Shell bool `json:"shell"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for CmdInstruction
func (c *CmdInstruction) GetCmd() string { return "CMD" }
func (c *CmdInstruction) GetArgs() []string { return c.Commands }
func (c *CmdInstruction) GetFlags() map[string]string { return nil }
func (c *CmdInstruction) GetLocation() *SourceLocation { return c.Location }
func (c *CmdInstruction) String() string {
	if len(c.Commands) > 0 {
		return "CMD " + c.Commands[0]
	}
	return "CMD"
}
func (c *CmdInstruction) Validate() error {
	if len(c.Commands) == 0 {
		return fmt.Errorf("CMD instruction requires at least one argument")
	}
	return nil
}

// EntrypointInstruction represents an ENTRYPOINT instruction.
type EntrypointInstruction struct {
	// Commands contains the command to execute
	Commands []string `json:"commands"`
	
	// Shell indicates if this is a shell form command
	Shell bool `json:"shell"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for EntrypointInstruction
func (e *EntrypointInstruction) GetCmd() string { return "ENTRYPOINT" }
func (e *EntrypointInstruction) GetArgs() []string { return e.Commands }
func (e *EntrypointInstruction) GetFlags() map[string]string { return nil }
func (e *EntrypointInstruction) GetLocation() *SourceLocation { return e.Location }
func (e *EntrypointInstruction) String() string {
	if len(e.Commands) > 0 {
		return "ENTRYPOINT " + e.Commands[0]
	}
	return "ENTRYPOINT"
}
func (e *EntrypointInstruction) Validate() error {
	if len(e.Commands) == 0 {
		return fmt.Errorf("ENTRYPOINT instruction requires at least one argument")
	}
	return nil
}

// WorkdirInstruction represents a WORKDIR instruction.
type WorkdirInstruction struct {
	// Path is the working directory path
	Path string `json:"path"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for WorkdirInstruction
func (w *WorkdirInstruction) GetCmd() string { return "WORKDIR" }
func (w *WorkdirInstruction) GetArgs() []string { return []string{w.Path} }
func (w *WorkdirInstruction) GetFlags() map[string]string { return nil }
func (w *WorkdirInstruction) GetLocation() *SourceLocation { return w.Location }
func (w *WorkdirInstruction) String() string { return "WORKDIR " + w.Path }
func (w *WorkdirInstruction) Validate() error {
	if w.Path == "" {
		return fmt.Errorf("WORKDIR instruction requires a path argument")
	}
	return nil
}

// UserInstruction represents a USER instruction.
type UserInstruction struct {
	// User is the username or UID
	User string `json:"user"`
	
	// Group is the optional group name or GID
	Group string `json:"group,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for UserInstruction
func (u *UserInstruction) GetCmd() string { return "USER" }
func (u *UserInstruction) GetArgs() []string {
	if u.Group != "" {
		return []string{u.User + ":" + u.Group}
	}
	return []string{u.User}
}
func (u *UserInstruction) GetFlags() map[string]string { return nil }
func (u *UserInstruction) GetLocation() *SourceLocation { return u.Location }
func (u *UserInstruction) String() string {
	if u.Group != "" {
		return "USER " + u.User + ":" + u.Group
	}
	return "USER " + u.User
}
func (u *UserInstruction) Validate() error {
	if u.User == "" {
		return fmt.Errorf("USER instruction requires a user argument")
	}
	return nil
}

// VolumeInstruction represents a VOLUME instruction.
type VolumeInstruction struct {
	// Paths contains the volume mount paths
	Paths []string `json:"paths"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for VolumeInstruction
func (v *VolumeInstruction) GetCmd() string { return "VOLUME" }
func (v *VolumeInstruction) GetArgs() []string { return v.Paths }
func (v *VolumeInstruction) GetFlags() map[string]string { return nil }
func (v *VolumeInstruction) GetLocation() *SourceLocation { return v.Location }
func (v *VolumeInstruction) String() string {
	if len(v.Paths) > 0 {
		return "VOLUME " + strings.Join(v.Paths, " ")
	}
	return "VOLUME"
}
func (v *VolumeInstruction) Validate() error {
	if len(v.Paths) == 0 {
		return fmt.Errorf("VOLUME instruction requires at least one path")
	}
	return nil
}

// ExposeInstruction represents an EXPOSE instruction.
type ExposeInstruction struct {
	// Ports contains the exposed ports
	Ports []string `json:"ports"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for ExposeInstruction
func (e *ExposeInstruction) GetCmd() string { return "EXPOSE" }
func (e *ExposeInstruction) GetArgs() []string { return e.Ports }
func (e *ExposeInstruction) GetFlags() map[string]string { return nil }
func (e *ExposeInstruction) GetLocation() *SourceLocation { return e.Location }
func (e *ExposeInstruction) String() string {
	if len(e.Ports) > 0 {
		return "EXPOSE " + strings.Join(e.Ports, " ")
	}
	return "EXPOSE"
}
func (e *ExposeInstruction) Validate() error {
	if len(e.Ports) == 0 {
		return fmt.Errorf("EXPOSE instruction requires at least one port")
	}
	
	// Validate port formats
	for _, port := range e.Ports {
		if err := validatePort(port); err != nil {
			return fmt.Errorf("invalid port format '%s': %w", port, err)
		}
	}
	return nil
}

// LabelInstruction represents a LABEL instruction.
type LabelInstruction struct {
	// Labels contains key-value pairs
	Labels map[string]string `json:"labels"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for LabelInstruction
func (l *LabelInstruction) GetCmd() string { return "LABEL" }
func (l *LabelInstruction) GetArgs() []string {
	args := make([]string, 0, len(l.Labels)*2)
	for k, v := range l.Labels {
		args = append(args, k+"="+v)
	}
	return args
}
func (l *LabelInstruction) GetFlags() map[string]string { return nil }
func (l *LabelInstruction) GetLocation() *SourceLocation { return l.Location }
func (l *LabelInstruction) String() string {
	if len(l.Labels) == 0 {
		return "LABEL"
	}
	for k, v := range l.Labels {
		return "LABEL " + k + "=" + v
	}
	return "LABEL"
}
func (l *LabelInstruction) Validate() error {
	if len(l.Labels) == 0 {
		return fmt.Errorf("LABEL instruction requires at least one label")
	}
	return nil
}

// ArgInstruction represents an ARG instruction.
type ArgInstruction struct {
	// Name is the argument name
	Name string `json:"name"`
	
	// DefaultValue is the optional default value
	DefaultValue string `json:"default_value,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for ArgInstruction
func (a *ArgInstruction) GetCmd() string { return "ARG" }
func (a *ArgInstruction) GetArgs() []string {
	if a.DefaultValue != "" {
		return []string{a.Name + "=" + a.DefaultValue}
	}
	return []string{a.Name}
}
func (a *ArgInstruction) GetFlags() map[string]string { return nil }
func (a *ArgInstruction) GetLocation() *SourceLocation { return a.Location }
func (a *ArgInstruction) String() string {
	if a.DefaultValue != "" {
		return "ARG " + a.Name + "=" + a.DefaultValue
	}
	return "ARG " + a.Name
}
func (a *ArgInstruction) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("ARG instruction requires a name")
	}
	return nil
}

// OnbuildInstruction represents an ONBUILD instruction.
type OnbuildInstruction struct {
	// Instruction is the instruction to execute on build
	Instruction Instruction `json:"instruction"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for OnbuildInstruction
func (o *OnbuildInstruction) GetCmd() string { return "ONBUILD" }
func (o *OnbuildInstruction) GetArgs() []string {
	if o.Instruction != nil {
		args := []string{o.Instruction.GetCmd()}
		args = append(args, o.Instruction.GetArgs()...)
		return args
	}
	return []string{}
}
func (o *OnbuildInstruction) GetFlags() map[string]string { return nil }
func (o *OnbuildInstruction) GetLocation() *SourceLocation { return o.Location }
func (o *OnbuildInstruction) String() string {
	if o.Instruction != nil {
		return "ONBUILD " + o.Instruction.String()
	}
	return "ONBUILD"
}
func (o *OnbuildInstruction) Validate() error {
	if o.Instruction == nil {
		return fmt.Errorf("ONBUILD instruction requires a sub-instruction")
	}
	
	// ONBUILD cannot contain certain instructions
	cmd := o.Instruction.GetCmd()
	if cmd == "FROM" || cmd == "ONBUILD" || cmd == "MAINTAINER" {
		return fmt.Errorf("ONBUILD instruction cannot contain %s", cmd)
	}
	
	return o.Instruction.Validate()
}

// StopsignalInstruction represents a STOPSIGNAL instruction.
type StopsignalInstruction struct {
	// Signal is the stop signal
	Signal string `json:"signal"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for StopsignalInstruction
func (s *StopsignalInstruction) GetCmd() string { return "STOPSIGNAL" }
func (s *StopsignalInstruction) GetArgs() []string { return []string{s.Signal} }
func (s *StopsignalInstruction) GetFlags() map[string]string { return nil }
func (s *StopsignalInstruction) GetLocation() *SourceLocation { return s.Location }
func (s *StopsignalInstruction) String() string { return "STOPSIGNAL " + s.Signal }
func (s *StopsignalInstruction) Validate() error {
	if s.Signal == "" {
		return fmt.Errorf("STOPSIGNAL instruction requires a signal")
	}
	return nil
}

// HealthcheckInstruction represents a HEALTHCHECK instruction.
type HealthcheckInstruction struct {
	// Type is the healthcheck type (NONE or CMD)
	Type string `json:"type"`
	
	// Commands contains the command to execute (for CMD type)
	Commands []string `json:"commands,omitempty"`
	
	// Interval is the check interval
	Interval string `json:"interval,omitempty"`
	
	// Timeout is the check timeout
	Timeout string `json:"timeout,omitempty"`
	
	// StartPeriod is the start period
	StartPeriod string `json:"start_period,omitempty"`
	
	// Retries is the number of retries
	Retries int `json:"retries,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for HealthcheckInstruction
func (h *HealthcheckInstruction) GetCmd() string { return "HEALTHCHECK" }
func (h *HealthcheckInstruction) GetArgs() []string {
	args := []string{h.Type}
	if h.Type == "CMD" {
		args = append(args, h.Commands...)
	}
	return args
}
func (h *HealthcheckInstruction) GetFlags() map[string]string {
	flags := make(map[string]string)
	if h.Interval != "" {
		flags["interval"] = h.Interval
	}
	if h.Timeout != "" {
		flags["timeout"] = h.Timeout
	}
	if h.StartPeriod != "" {
		flags["start-period"] = h.StartPeriod
	}
	if h.Retries > 0 {
		flags["retries"] = strconv.Itoa(h.Retries)
	}
	return flags
}
func (h *HealthcheckInstruction) GetLocation() *SourceLocation { return h.Location }
func (h *HealthcheckInstruction) String() string {
	return "HEALTHCHECK " + h.Type
}
func (h *HealthcheckInstruction) Validate() error {
	if h.Type != "NONE" && h.Type != "CMD" {
		return fmt.Errorf("HEALTHCHECK type must be NONE or CMD")
	}
	if h.Type == "CMD" && len(h.Commands) == 0 {
		return fmt.Errorf("HEALTHCHECK CMD requires at least one command")
	}
	return nil
}

// ShellInstruction represents a SHELL instruction.
type ShellInstruction struct {
	// Shell contains the shell command and arguments
	Shell []string `json:"shell"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for ShellInstruction
func (s *ShellInstruction) GetCmd() string { return "SHELL" }
func (s *ShellInstruction) GetArgs() []string { return s.Shell }
func (s *ShellInstruction) GetFlags() map[string]string { return nil }
func (s *ShellInstruction) GetLocation() *SourceLocation { return s.Location }
func (s *ShellInstruction) String() string {
	if len(s.Shell) > 0 {
		return "SHELL " + strings.Join(s.Shell, " ")
	}
	return "SHELL"
}
func (s *ShellInstruction) Validate() error {
	if len(s.Shell) == 0 {
		return fmt.Errorf("SHELL instruction requires at least one argument")
	}
	return nil
}

// Helper function to validate port format
func validatePort(port string) error {
	// Port can be just a number or number/protocol
	parts := strings.Split(port, "/")
	
	// Parse port number
	portNum, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid port number: %s", parts[0])
	}
	
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port number out of range: %d", portNum)
	}
	
	// Validate protocol if specified
	if len(parts) > 1 {
		protocol := strings.ToLower(parts[1])
		if protocol != "tcp" && protocol != "udp" {
			return fmt.Errorf("invalid protocol: %s", parts[1])
		}
	}
	
	return nil
}