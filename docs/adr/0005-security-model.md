# ADR-0005: Security Model

## Status
Accepted

## Context

Security is a fundamental requirement for shmocker, as outlined in the PRD. The system must:

1. **Run rootless** - No elevated privileges required
2. **Generate SBOMs** - Software Bill of Materials for supply chain transparency  
3. **Sign images** - Cryptographic signatures using Cosign
4. **Secure by design** - Minimal attack surface and defense in depth
5. **Supply chain security** - Attestations and provenance tracking

Key security challenges include:

### Rootless Execution Security
- User namespace isolation
- Container breakout prevention
- Resource access controls
- Privilege escalation prevention

### Supply Chain Security
- Build provenance tracking
- Dependency vulnerability scanning
- SBOM generation and signing
- Attestation creation and verification

### Cryptographic Operations
- Key management and storage
- Image signing with Cosign
- Certificate handling
- Secure random generation

### Network Security
- Registry authentication
- TLS certificate validation
- Man-in-the-middle prevention
- DNS security

## Decision

We will implement a **comprehensive security model** with:

1. **Zero-Trust Architecture**: Assume all inputs are untrusted
2. **Defense in Depth**: Multiple layers of security controls
3. **Principle of Least Privilege**: Minimal required permissions
4. **Supply Chain Security**: SLSA-compliant build attestations
5. **Cryptographic Integrity**: All artifacts signed and verified

## Architecture Design

### Security Framework Interface

```go
type SecurityManager interface {
    // Access control
    AuthorizeOperation(ctx context.Context, operation string, resource string, user *User) error
    ValidateInput(ctx context.Context, input interface{}) error
    
    // Cryptographic operations
    SignArtifact(ctx context.Context, artifact []byte, keyRef string) (*Signature, error)
    VerifySignature(ctx context.Context, artifact []byte, signature *Signature) error
    
    // Supply chain security
    GenerateProvenance(ctx context.Context, buildInfo *BuildInfo) (*Provenance, error)
    VerifySupplyChain(ctx context.Context, imageRef string) (*SupplyChainResult, error)
    
    // Security policies
    EnforcePolicy(ctx context.Context, policy *SecurityPolicy, context *SecurityContext) error
    AuditOperation(ctx context.Context, operation *AuditEvent) error
}
```

### Rootless Security Model

```go
type RootlessSecurityConfig struct {
    // User namespace configuration
    UserNamespace *UserNamespaceConfig `json:"user_namespace"`
    
    // Security constraints
    NoNewPrivileges bool     `json:"no_new_privileges"`
    Seccomp        string    `json:"seccomp,omitempty"`
    AppArmor       string    `json:"apparmor,omitempty"`
    SELinux        string    `json:"selinux,omitempty"`
    
    // Resource limits
    ResourceLimits *ResourceLimits `json:"resource_limits"`
    
    // Network restrictions
    NetworkPolicy *NetworkPolicy `json:"network_policy"`
}

type UserNamespaceConfig struct {
    // UID/GID mapping
    UIDMap []IDMap `json:"uid_map"`
    GIDMap []IDMap `json:"gid_map"`
    
    // Capability restrictions
    DropCapabilities []string `json:"drop_capabilities"`
    KeepCapabilities []string `json:"keep_capabilities,omitempty"`
    
    // Mount restrictions
    AllowedMounts []string `json:"allowed_mounts,omitempty"`
    ReadOnlyPaths []string `json:"readonly_paths,omitempty"`
}

type ResourceLimits struct {
    Memory      int64         `json:"memory,omitempty"`
    CPU         string        `json:"cpu,omitempty"`
    Processes   int           `json:"processes,omitempty"`
    FileHandles int          `json:"file_handles,omitempty"`
    Timeout     time.Duration `json:"timeout,omitempty"`
}
```

### Supply Chain Security Framework

```go
type SupplyChainManager interface {
    // SBOM operations
    GenerateSBOM(ctx context.Context, imageRef string, opts *SBOMOptions) (*SBOM, error)
    VerifySBOM(ctx context.Context, sbom *SBOM) (*SBOMVerificationResult, error)
    AttachSBOM(ctx context.Context, imageRef string, sbom *SBOM) error
    
    // Provenance operations
    GenerateProvenance(ctx context.Context, buildInfo *BuildInfo) (*Provenance, error)
    VerifyProvenance(ctx context.Context, provenance *Provenance) (*ProvenanceVerificationResult, error)
    AttachProvenance(ctx context.Context, imageRef string, provenance *Provenance) error
    
    // Vulnerability scanning
    ScanVulnerabilities(ctx context.Context, sbom *SBOM) (*VulnerabilityReport, error)
    CheckPolicies(ctx context.Context, report *VulnerabilityReport, policies []SecurityPolicy) (*PolicyResult, error)
}

// SLSA Provenance v0.2 compliant structure
type Provenance struct {
    Builder     Builder     `json:"builder"`
    BuildType   string      `json:"buildType"`
    Invocation  Invocation  `json:"invocation"`
    BuildConfig BuildConfig `json:"buildConfig"`
    Metadata    Metadata    `json:"metadata"`
    Materials   []Material  `json:"materials"`
}

type Builder struct {
    ID      string            `json:"id"`
    Version map[string]string `json:"version,omitempty"`
}

type Invocation struct {
    ConfigSource ConfigSource `json:"configSource"`
    Parameters   interface{}  `json:"parameters,omitempty"`
    Environment  interface{}  `json:"environment,omitempty"`
}

type Material struct {
    URI    string            `json:"uri"`
    Digest map[string]string `json:"digest"`
}
```

### Cryptographic Security

```go
type CryptographicManager interface {
    // Key management
    GenerateKeyPair(ctx context.Context, algorithm string, opts *KeyGenOptions) (*KeyPair, error)
    StoreKey(ctx context.Context, keyID string, privateKey crypto.PrivateKey) error
    LoadKey(ctx context.Context, keyID string) (crypto.PrivateKey, error)
    
    // Signing operations
    SignData(ctx context.Context, data []byte, keyID string) (*Signature, error)
    VerifySignature(ctx context.Context, data []byte, signature *Signature, publicKey crypto.PublicKey) error
    
    // Certificate operations
    GenerateCSR(ctx context.Context, template *x509.CertificateRequest, privateKey crypto.PrivateKey) ([]byte, error)
    ValidateCertificate(ctx context.Context, cert *x509.Certificate) (*CertificateValidationResult, error)
    
    // Secure random generation
    GenerateRandomBytes(length int) ([]byte, error)
    GenerateNonce() (string, error)
}

// Secure key storage abstraction
type KeyStore interface {
    Store(ctx context.Context, keyID string, key crypto.PrivateKey, metadata map[string]string) error
    Load(ctx context.Context, keyID string) (crypto.PrivateKey, error)
    Delete(ctx context.Context, keyID string) error
    List(ctx context.Context) ([]KeyInfo, error)
    Exists(ctx context.Context, keyID string) (bool, error)
}

// Support multiple key storage backends
type KeyStoreType string

const (
    KeyStoreTypeFile     KeyStoreType = "file"      // Encrypted files
    KeyStoreTypeHSM      KeyStoreType = "hsm"       // Hardware Security Module
    KeyStoreTypeKMS      KeyStoreType = "kms"       // Cloud KMS (AWS, GCP, Azure)
    KeyStoreTypeVault    KeyStoreType = "vault"     // HashiCorp Vault
    KeyStoreTypeKeychain KeyStoreType = "keychain"  // OS keychain/keyring
)
```

### Input Validation and Sanitization

```go
type InputValidator interface {
    ValidateDockerfile(content []byte) (*ValidationResult, error)
    ValidateBuildContext(path string) (*ValidationResult, error)
    ValidateImageReference(ref string) (*ValidationResult, error)
    ValidateBuildArgs(args map[string]string) (*ValidationResult, error)
    SanitizeUserInput(input string) (string, error)
}

type ValidationResult struct {
    Valid    bool      `json:"valid"`
    Errors   []string  `json:"errors,omitempty"`
    Warnings []string  `json:"warnings,omitempty"`
    Sanitized interface{} `json:"sanitized,omitempty"`
}

// Dockerfile security validation rules
type DockerfileSecurityRules struct {
    // Prohibited instructions
    DisallowedInstructions []string `json:"disallowed_instructions"`
    
    // Security policy violations
    RequireNonRootUser     bool     `json:"require_non_root_user"`
    DisallowPrivileged     bool     `json:"disallow_privileged"`
    MaxLayers              int      `json:"max_layers,omitempty"`
    
    // Resource limits
    MaxBuildArgs           int      `json:"max_build_args,omitempty"`
    MaxEnvVars            int      `json:"max_env_vars,omitempty"`
    
    // Content restrictions
    DisallowedPackages    []string `json:"disallowed_packages,omitempty"`
    RequiredLabels        []string `json:"required_labels,omitempty"`
}
```

### Network Security

```go
type NetworkSecurityManager interface {
    // TLS configuration
    ConfigureTLS(ctx context.Context, config *TLSConfig) (*tls.Config, error)
    ValidateCertificate(ctx context.Context, cert *x509.Certificate, hostname string) error
    
    // Registry security
    SecureRegistryConnection(ctx context.Context, registry string) (*SecureConnection, error)
    ValidateRegistryAuth(ctx context.Context, auth *AuthConfig) error
    
    // DNS security
    SecureDNSLookup(ctx context.Context, hostname string) ([]string, error)
    ValidateDNSResponse(ctx context.Context, response *dns.Msg) error
}

type TLSConfig struct {
    MinVersion            uint16   `json:"min_version"`
    CipherSuites         []uint16 `json:"cipher_suites,omitempty"`
    InsecureSkipVerify   bool     `json:"insecure_skip_verify,omitempty"`
    ClientCertificate    string   `json:"client_certificate,omitempty"`
    ClientKey            string   `json:"client_key,omitempty"`
    CACertificates       []string `json:"ca_certificates,omitempty"`
    VerifyServerName     bool     `json:"verify_server_name"`
}

type SecureConnection struct {
    Transport    *http.Transport `json:"-"`
    TLSConfig    *tls.Config     `json:"-"`
    Timeout      time.Duration   `json:"timeout"`
    RetryPolicy  *RetryPolicy    `json:"retry_policy"`
}
```

## Security Policies and Governance

### Policy Engine

```go
type PolicyEngine interface {
    // Policy management
    LoadPolicy(ctx context.Context, source string) (*SecurityPolicy, error)
    ValidatePolicy(ctx context.Context, policy *SecurityPolicy) error
    
    // Policy evaluation
    EvaluatePolicy(ctx context.Context, policy *SecurityPolicy, context *SecurityContext) (*PolicyResult, error)
    
    // Policy enforcement
    EnforcePolicy(ctx context.Context, result *PolicyResult) error
}

type SecurityPolicy struct {
    Version     string           `json:"version"`
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Rules       []PolicyRule     `json:"rules"`
    Enforcement EnforcementMode  `json:"enforcement"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

type PolicyRule struct {
    ID          string      `json:"id"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Condition   string      `json:"condition"`   // OPA Rego expression
    Action      PolicyAction `json:"action"`
    Severity    string      `json:"severity"`
    Message     string      `json:"message,omitempty"`
}

type EnforcementMode string

const (
    EnforcementModeWarn    EnforcementMode = "warn"    // Log warnings
    EnforcementModeBlock   EnforcementMode = "block"   // Block operation
    EnforcementModeMonitor EnforcementMode = "monitor" // Log for monitoring
)

// Example security policies
var DefaultSecurityPolicies = []SecurityPolicy{
    {
        Name: "dockerfile-security",
        Rules: []PolicyRule{
            {
                ID:          "no-root-user",
                Name:        "Prohibit root user",
                Description: "Container should not run as root user",
                Condition:   `input.dockerfile.user == "root" or input.dockerfile.user == "0" or not input.dockerfile.user`,
                Action:      PolicyActionBlock,
                Severity:    "HIGH",
                Message:     "Container must not run as root user. Add USER instruction with non-root user.",
            },
            {
                ID:          "no-privileged",
                Name:        "Prohibit privileged containers",
                Description: "Container should not require privileged mode",
                Condition:   `input.dockerfile contains "privileged"`,
                Action:      PolicyActionBlock,
                Severity:    "CRITICAL",
                Message:     "Privileged containers are not allowed.",
            },
        },
        Enforcement: EnforcementModeBlock,
    },
}
```

### Audit and Compliance

```go
type AuditManager interface {
    // Audit logging
    LogAuditEvent(ctx context.Context, event *AuditEvent) error
    GetAuditLog(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error)
    
    // Compliance reporting
    GenerateComplianceReport(ctx context.Context, framework string, timeRange TimeRange) (*ComplianceReport, error)
    ValidateCompliance(ctx context.Context, framework string) (*ComplianceResult, error)
}

type AuditEvent struct {
    ID          string                 `json:"id"`
    Timestamp   time.Time              `json:"timestamp"`
    UserID      string                 `json:"user_id,omitempty"`
    SessionID   string                 `json:"session_id,omitempty"`
    Operation   string                 `json:"operation"`
    Resource    string                 `json:"resource"`
    Result      AuditResult            `json:"result"`
    Details     map[string]interface{} `json:"details,omitempty"`
    ClientIP    string                 `json:"client_ip,omitempty"`
    UserAgent   string                 `json:"user_agent,omitempty"`
}

type AuditResult string

const (
    AuditResultSuccess AuditResult = "success"
    AuditResultFailure AuditResult = "failure"
    AuditResultDenied  AuditResult = "denied"
)
```

## Vulnerability Scanning Integration

### Scanner Interface

```go
type VulnerabilityScanner interface {
    // Image scanning
    ScanImage(ctx context.Context, imageRef string) (*VulnerabilityReport, error)
    ScanSBOM(ctx context.Context, sbom *SBOM) (*VulnerabilityReport, error)
    
    // Database operations
    UpdateDatabase(ctx context.Context) error
    GetDatabaseInfo(ctx context.Context) (*DatabaseInfo, error)
    
    // Configuration
    Configure(ctx context.Context, config *ScannerConfig) error
}

type VulnerabilityReport struct {
    ImageRef      string          `json:"image_ref"`
    ScanTime      time.Time       `json:"scan_time"`
    Scanner       ScannerInfo     `json:"scanner"`
    Summary       VulnSummary     `json:"summary"`
    Vulnerabilities []Vulnerability `json:"vulnerabilities"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type Vulnerability struct {
    ID           string            `json:"id"`
    PackageName  string            `json:"package_name"`
    Version      string            `json:"version"`
    FixedVersion string            `json:"fixed_version,omitempty"`
    Severity     VulnSeverity      `json:"severity"`
    Score        float64           `json:"score,omitempty"`
    Vector       string            `json:"vector,omitempty"`
    Description  string            `json:"description"`
    References   []string          `json:"references,omitempty"`
    PublishedAt  *time.Time        `json:"published_at,omitempty"`
    ModifiedAt   *time.Time        `json:"modified_at,omitempty"`
}

type VulnSeverity string

const (
    VulnSeverityUnknown    VulnSeverity = "UNKNOWN"
    VulnSeverityNegligible VulnSeverity = "NEGLIGIBLE" 
    VulnSeverityLow        VulnSeverity = "LOW"
    VulnSeverityMedium     VulnSeverity = "MEDIUM"
    VulnSeverityHigh       VulnSeverity = "HIGH"
    VulnSeverityCritical   VulnSeverity = "CRITICAL"
)
```

## Implementation Security Measures

### Secure Coding Practices

```go
// Example of secure input handling
func ValidateImageReference(ref string) error {
    // Prevent injection attacks
    if strings.Contains(ref, ";") || strings.Contains(ref, "&") || strings.Contains(ref, "|") {
        return errors.New("invalid characters in image reference")
    }
    
    // Validate format
    if !regexp.MustCompile(`^[a-zA-Z0-9._/-]+:?[a-zA-Z0-9._-]*$`).MatchString(ref) {
        return errors.New("invalid image reference format")
    }
    
    // Length limits
    if len(ref) > 255 {
        return errors.New("image reference too long")
    }
    
    return nil
}

// Secure temporary file handling
func CreateSecureTempFile(prefix string) (*os.File, error) {
    // Create with restrictive permissions
    file, err := os.CreateTemp("", prefix)
    if err != nil {
        return nil, err
    }
    
    // Set secure permissions (owner read/write only)
    if err := file.Chmod(0600); err != nil {
        file.Close()
        os.Remove(file.Name())
        return nil, err
    }
    
    return file, nil
}
```

### Resource Protection

```go
type ResourceManager interface {
    // Resource limits
    SetMemoryLimit(limit int64) error
    SetCPULimit(limit string) error
    SetProcessLimit(limit int) error
    SetTimeLimit(limit time.Duration) error
    
    // Resource monitoring
    GetResourceUsage() (*ResourceUsage, error)
    EnforceResourceLimits() error
}

type ResourceUsage struct {
    Memory    int64         `json:"memory"`
    CPU       float64       `json:"cpu"`
    Processes int           `json:"processes"`
    Runtime   time.Duration `json:"runtime"`
}
```

## Security Testing Strategy

### Security Test Framework

```go
func TestSecurityValidation(t *testing.T) {
    testCases := []struct {
        name           string
        dockerfile     string
        expectViolation bool
        expectedRule   string
    }{
        {
            name: "root-user-violation",
            dockerfile: `
FROM alpine
USER root
CMD ["/bin/sh"]
`,
            expectViolation: true,
            expectedRule:   "no-root-user",
        },
        {
            name: "non-root-user-allowed",
            dockerfile: `
FROM alpine
RUN adduser -D appuser
USER appuser
CMD ["/bin/sh"]
`,
            expectViolation: false,
        },
    }
    
    policyEngine := NewPolicyEngine()
    policy := LoadSecurityPolicy("default")
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            context := &SecurityContext{
                Dockerfile: tc.dockerfile,
            }
            
            result, err := policyEngine.EvaluatePolicy(context, policy)
            assert.NoError(t, err)
            
            if tc.expectViolation {
                assert.True(t, result.HasViolations())
                assert.Contains(t, result.ViolatedRules, tc.expectedRule)
            } else {
                assert.False(t, result.HasViolations())
            }
        })
    }
}
```

## Consequences

### Positive

1. **Strong Security Posture**: Comprehensive protection against common attack vectors
2. **Supply Chain Transparency**: SBOM and provenance tracking
3. **Compliance Ready**: Built-in audit trails and policy enforcement
4. **Zero-Trust Architecture**: Assume-breach mindset with defense in depth
5. **Cryptographic Integrity**: All artifacts signed and verifiable

### Negative

1. **Implementation Complexity**: Comprehensive security requires significant engineering effort
2. **Performance Impact**: Security checks and cryptographic operations add overhead
3. **User Experience**: Additional security steps may impact usability
4. **Dependency Management**: Security scanning and signing require external services

### Mitigation Strategies

1. **Progressive Implementation**: Start with core security features and expand
2. **Performance Optimization**: Cache security validations and optimize hot paths
3. **User Experience Design**: Make security features transparent where possible
4. **Fallback Options**: Graceful degradation when external services unavailable

## Implementation Roadmap

### Phase 1: Core Security Foundation
- Rootless execution hardening
- Input validation and sanitization
- Basic cryptographic operations
- Audit logging

### Phase 2: Supply Chain Security
- SBOM generation and signing
- Provenance attestation
- Policy engine implementation
- Vulnerability scanning integration

### Phase 3: Advanced Security Features
- HSM/KMS integration
- Advanced threat detection
- Security analytics and monitoring
- Compliance reporting

### Phase 4: Enterprise Security
- SSO integration
- Advanced policy management
- Security orchestration
- Threat intelligence integration

## References

- [SLSA Framework](https://slsa.dev/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [OWASP Container Security Guide](https://owasp.org/www-project-container-security/)
- [Sigstore Documentation](https://docs.sigstore.dev/)
- [SPDX Specification](https://spdx.github.io/spdx-spec/)

## Related ADRs

- [ADR-0001: Embed BuildKit as Library](./0001-embed-buildkit-as-library.md)
- [ADR-0002: Rootless Execution Strategy](./0002-rootless-execution-strategy.md)
- [ADR-0004: Error Handling Strategy](./0004-error-handling-strategy.md)