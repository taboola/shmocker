package builder

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
)

// ContextManager handles different types of build contexts
type ContextManager struct {
	tempDir string
}

// NewContextManager creates a new context manager
func NewContextManager() (*ContextManager, error) {
	tempDir, err := os.MkdirTemp("", "shmocker-context-*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary directory")
	}

	return &ContextManager{
		tempDir: tempDir,
	}, nil
}

// PrepareContext prepares a build context based on its type
func (cm *ContextManager) PrepareContext(ctx context.Context, buildCtx *BuildContext) (*PreparedContext, error) {
	switch buildCtx.Type {
	case ContextTypeLocal:
		return cm.prepareLocalContext(ctx, buildCtx)
	case ContextTypeGit:
		return cm.prepareGitContext(ctx, buildCtx)
	case ContextTypeTar:
		return cm.prepareTarContext(ctx, buildCtx)
	case ContextTypeHTTP:
		return cm.prepareHTTPContext(ctx, buildCtx)
	case ContextTypeStdin:
		return cm.prepareStdinContext(ctx, buildCtx)
	default:
		return nil, errors.Errorf("unsupported context type: %s", buildCtx.Type)
	}
}

// Close cleans up temporary resources
func (cm *ContextManager) Close() error {
	if cm.tempDir != "" {
		return os.RemoveAll(cm.tempDir)
	}
	return nil
}

// PreparedContext represents a prepared build context ready for use
type PreparedContext struct {
	Type         ContextType
	LocalPath    string
	Session      session.Attachable
	CleanupFunc  func() error
	ExcludeFunc  fsutil.FilterFunc
	DockerIgnore bool
}

// Close cleans up the prepared context
func (pc *PreparedContext) Close() error {
	if pc.CleanupFunc != nil {
		return pc.CleanupFunc()
	}
	return nil
}

// prepareLocalContext prepares a local directory context
func (cm *ContextManager) prepareLocalContext(ctx context.Context, buildCtx *BuildContext) (*PreparedContext, error) {
	absPath, err := filepath.Abs(buildCtx.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get absolute path")
	}

	// Verify directory exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, errors.Wrap(err, "build context directory does not exist")
	}

	// Create exclude function
	excludeFunc, err := cm.createExcludeFunc(absPath, buildCtx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exclude function")
	}

	// Create filesync session
	syncProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{
			Name: "context",
			Dir:  absPath,
			Map:  cm.createPathMap(absPath),
		},
	})

	return &PreparedContext{
		Type:         ContextTypeLocal,
		LocalPath:    absPath,
		Session:      syncProvider,
		ExcludeFunc:  excludeFunc,
		DockerIgnore: buildCtx.DockerIgnore,
	}, nil
}

// prepareGitContext prepares a Git repository context
func (cm *ContextManager) prepareGitContext(ctx context.Context, buildCtx *BuildContext) (*PreparedContext, error) {
	// Parse Git URL and options
	gitURL, gitRef, gitSubdir, err := parseGitURL(buildCtx.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse Git URL")
	}

	// Create temporary directory for Git checkout
	gitDir := filepath.Join(cm.tempDir, "git-context")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create Git context directory")
	}

	// Clone the repository
	if err := cm.cloneGitRepository(ctx, gitURL, gitRef, gitDir); err != nil {
		return nil, errors.Wrap(err, "failed to clone Git repository")
	}

	// Handle subdirectory if specified
	contextPath := gitDir
	if gitSubdir != "" {
		contextPath = filepath.Join(gitDir, gitSubdir)
		if _, err := os.Stat(contextPath); err != nil {
			return nil, errors.Wrapf(err, "Git subdirectory %s does not exist", gitSubdir)
		}
	}

	// Create exclude function
	excludeFunc, err := cm.createExcludeFunc(contextPath, buildCtx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exclude function")
	}

	// Create filesync session
	syncProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{
			Name: "context",
			Dir:  contextPath,
			Map:  cm.createPathMap(contextPath),
		},
	})

	return &PreparedContext{
		Type:         ContextTypeGit,
		LocalPath:    contextPath,
		Session:      syncProvider,
		ExcludeFunc:  excludeFunc,
		DockerIgnore: buildCtx.DockerIgnore,
		CleanupFunc: func() error {
			return os.RemoveAll(gitDir)
		},
	}, nil
}

// prepareTarContext prepares a tar archive context
func (cm *ContextManager) prepareTarContext(ctx context.Context, buildCtx *BuildContext) (*PreparedContext, error) {
	// Create temporary directory for tar extraction
	tarDir := filepath.Join(cm.tempDir, "tar-context")
	if err := os.MkdirAll(tarDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create tar context directory")
	}

	// Open tar file
	tarFile, err := os.Open(buildCtx.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open tar file")
	}
	defer tarFile.Close()

	// Extract tar archive
	if err := cm.extractTarArchive(tarFile, tarDir); err != nil {
		return nil, errors.Wrap(err, "failed to extract tar archive")
	}

	// Create exclude function
	excludeFunc, err := cm.createExcludeFunc(tarDir, buildCtx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exclude function")
	}

	// Create filesync session
	syncProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{
			Name: "context",
			Dir:  tarDir,
			Map:  cm.createPathMap(tarDir),
		},
	})

	return &PreparedContext{
		Type:         ContextTypeTar,
		LocalPath:    tarDir,
		Session:      syncProvider,
		ExcludeFunc:  excludeFunc,
		DockerIgnore: buildCtx.DockerIgnore,
		CleanupFunc: func() error {
			return os.RemoveAll(tarDir)
		},
	}, nil
}

// prepareHTTPContext prepares a HTTP URL context
func (cm *ContextManager) prepareHTTPContext(ctx context.Context, buildCtx *BuildContext) (*PreparedContext, error) {
	// Create temporary directory for HTTP download
	httpDir := filepath.Join(cm.tempDir, "http-context")
	if err := os.MkdirAll(httpDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP context directory")
	}

	// Download file from HTTP URL
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(buildCtx.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to download from HTTP URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	// Determine if it's a tar archive based on content type or URL
	isArchive := strings.Contains(resp.Header.Get("Content-Type"), "tar") ||
		strings.HasSuffix(buildCtx.Source, ".tar") ||
		strings.HasSuffix(buildCtx.Source, ".tar.gz") ||
		strings.HasSuffix(buildCtx.Source, ".tgz")

	var contextPath string
	if isArchive {
		// Extract as tar archive
		if err := cm.extractTarArchive(resp.Body, httpDir); err != nil {
			return nil, errors.Wrap(err, "failed to extract HTTP tar archive")
		}
		contextPath = httpDir
	} else {
		// Save as single file
		filename := filepath.Base(buildCtx.Source)
		if filename == "" || filename == "." {
			filename = "Dockerfile"
		}
		filePath := filepath.Join(httpDir, filename)
		
		file, err := os.Create(filePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create file")
		}
		defer file.Close()

		if _, err := io.Copy(file, resp.Body); err != nil {
			return nil, errors.Wrap(err, "failed to save HTTP content")
		}
		contextPath = httpDir
	}

	// Create exclude function
	excludeFunc, err := cm.createExcludeFunc(contextPath, buildCtx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exclude function")
	}

	// Create filesync session
	syncProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{
			Name: "context",
			Dir:  contextPath,
			Map:  cm.createPathMap(contextPath),
		},
	})

	return &PreparedContext{
		Type:         ContextTypeHTTP,
		LocalPath:    contextPath,
		Session:      syncProvider,
		ExcludeFunc:  excludeFunc,
		DockerIgnore: buildCtx.DockerIgnore,
		CleanupFunc: func() error {
			return os.RemoveAll(httpDir)
		},
	}, nil
}

// prepareStdinContext prepares a stdin context
func (cm *ContextManager) prepareStdinContext(ctx context.Context, buildCtx *BuildContext) (*PreparedContext, error) {
	// Create temporary directory for stdin content
	stdinDir := filepath.Join(cm.tempDir, "stdin-context")
	if err := os.MkdirAll(stdinDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create stdin context directory")
	}

	// For stdin context, we expect the Dockerfile content to be provided directly
	// This is typically used when piping Dockerfile content to the build command
	dockerfilePath := filepath.Join(stdinDir, "Dockerfile")
	dockerfileFile, err := os.Create(dockerfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Dockerfile")
	}
	defer dockerfileFile.Close()

	// Copy stdin content to Dockerfile
	if _, err := io.Copy(dockerfileFile, os.Stdin); err != nil {
		return nil, errors.Wrap(err, "failed to read from stdin")
	}

	// Create exclude function
	excludeFunc, err := cm.createExcludeFunc(stdinDir, buildCtx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create exclude function")
	}

	// Create filesync session
	syncProvider := filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{
			Name: "context",
			Dir:  stdinDir,
			Map:  cm.createPathMap(stdinDir),
		},
	})

	return &PreparedContext{
		Type:         ContextTypeStdin,
		LocalPath:    stdinDir,
		Session:      syncProvider,
		ExcludeFunc:  excludeFunc,
		DockerIgnore: false, // No .dockerignore for stdin context
		CleanupFunc: func() error {
			return os.RemoveAll(stdinDir)
		},
	}, nil
}

// createExcludeFunc creates a filter function for excluding files
func (cm *ContextManager) createExcludeFunc(contextPath string, buildCtx *BuildContext) (fsutil.FilterFunc, error) {
	var patterns []string

	// Add explicit excludes
	patterns = append(patterns, buildCtx.Exclude...)

	// Parse .dockerignore if enabled
	if buildCtx.DockerIgnore {
		dockerignorePath := filepath.Join(contextPath, ".dockerignore")
		if _, err := os.Stat(dockerignorePath); err == nil {
			ignorePatterns, err := parseDockerignore(dockerignorePath)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse .dockerignore")
			}
			patterns = append(patterns, ignorePatterns...)
		}
	}

	// Create filter function
	return fsutil.FilterFunc(func(path string, info os.FileInfo) bool {
		// Always include if no patterns
		if len(patterns) == 0 {
			return true
		}

		// Check against each pattern
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, path)
			if err != nil {
				continue // Invalid pattern, skip
			}
			if matched {
				return false // Exclude if matched
			}
		}

		return true // Include by default
	}), nil
}

// createPathMap creates a path mapping function for the filesync session
func (cm *ContextManager) createPathMap(contextPath string) func(string, os.FileInfo) (string, error) {
	return func(path string, info os.FileInfo) (string, error) {
		// Return relative path from context root
		relPath, err := filepath.Rel(contextPath, path)
		if err != nil {
			return "", err
		}
		return relPath, nil
	}
}

// parseGitURL parses a Git URL and extracts URL, ref, and subdirectory
func parseGitURL(gitSpec string) (url, ref, subdir string, err error) {
	// Handle Git URL format: git://url#ref:subdir
	parts := strings.SplitN(gitSpec, "#", 2)
	url = parts[0]

	if len(parts) == 2 {
		// Parse ref and subdir
		refParts := strings.SplitN(parts[1], ":", 2)
		ref = refParts[0]
		if len(refParts) == 2 {
			subdir = refParts[1]
		}
	}

	if url == "" {
		err = errors.New("empty Git URL")
	}

	return
}

// cloneGitRepository clones a Git repository to the specified directory
func (cm *ContextManager) cloneGitRepository(ctx context.Context, gitURL, gitRef, targetDir string) error {
	// TODO: Implement Git cloning
	// This would use a Git library or shell out to git command
	// For now, return an error indicating it's not implemented
	return errors.New("Git context cloning not yet implemented")
}

// extractTarArchive extracts a tar archive to the specified directory
func (cm *ContextManager) extractTarArchive(r io.Reader, targetDir string) error {
	tarReader := tar.NewReader(r)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read tar header")
		}

		// Security check: prevent path traversal
		if strings.Contains(header.Name, "..") {
			return errors.Errorf("invalid path in tar archive: %s", header.Name)
		}

		targetPath := filepath.Join(targetDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return errors.Wrapf(err, "failed to create directory %s", targetPath)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return errors.Wrapf(err, "failed to create parent directory for %s", targetPath)
			}

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", targetPath)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return errors.Wrapf(err, "failed to write file %s", targetPath)
			}
			file.Close()
		}
	}

	return nil
}

// parseDockerignore parses a .dockerignore file and returns exclusion patterns
func parseDockerignore(dockerignorePath string) ([]string, error) {
	file, err := os.Open(dockerignorePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := fsutil.NewScanner(file)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}