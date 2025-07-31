package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/shmocker/shmocker/internal/config"
	"github.com/shmocker/shmocker/pkg/builder"
	"github.com/shmocker/shmocker/pkg/dockerfile"
)

var (
	// Version information (set by build)
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"

	// Global flags
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "shmocker",
	Short: "A rootless Docker image builder",
	Long: `Shmocker is a rootless Docker image builder that provides a secure and 
efficient way to build container images without requiring root privileges.

Features:
- Rootless container image building
- OCI-compliant image output
- SBOM generation
- Image signing with Cosign
- Multi-stage build support
- Build caching`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			fmt.Printf("Shmocker version: %s\n", version)
			fmt.Printf("Git commit: %s\n", commit)
			fmt.Printf("Build time: %s\n", buildTime)
		}
	},
}

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build [flags] PATH",
	Short: "Build a container image",
	Long: `Build a container image from a Dockerfile in the specified path.
The build context will be the specified directory.`,
	Args: cobra.ExactArgs(1),
	RunE: runBuildCommand,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Shmocker version: %s\n", version)
		fmt.Printf("Git commit: %s\n", commit)
		fmt.Printf("Build time: %s\n", buildTime)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.shmocker.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Build command flags - standard Docker build flags
	buildCmd.Flags().StringSliceP("tag", "t", []string{}, "name and optionally a tag in the 'name:tag' format")
	buildCmd.Flags().StringP("file", "f", "Dockerfile", "name of the Dockerfile (default is 'Dockerfile')")
	buildCmd.Flags().Bool("no-cache", false, "do not use cache when building the image")
	buildCmd.Flags().Bool("pull", false, "always attempt to pull a newer version of the image")
	buildCmd.Flags().StringSlice("build-arg", []string{}, "set build-time variables")
	buildCmd.Flags().StringSlice("label", []string{}, "set metadata for an image")
	buildCmd.Flags().StringSlice("platform", []string{}, "set target platform for build")
	buildCmd.Flags().String("target", "", "set the target build stage to build")
	buildCmd.Flags().StringSlice("cache-from", []string{}, "images to consider as cache sources")
	buildCmd.Flags().StringSlice("cache-to", []string{}, "cache export destinations")
	buildCmd.Flags().String("network", "default", "set the networking mode for the RUN instructions during build")
	buildCmd.Flags().String("progress", "auto", "set type of progress output (auto, plain, tty)")
	buildCmd.Flags().String("output", "", "output destination (format: type=local,dest=path)")
	buildCmd.Flags().Bool("quiet", false, "suppress the build output and print image ID on success")

	// Shmocker-specific flags
	buildCmd.Flags().Bool("sbom", false, "generate SBOM for the image")
	buildCmd.Flags().Bool("sign", false, "sign the image with Cosign")

	// Add subcommands
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(versionCmd)

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".shmocker" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".shmocker")
	}

	// Environment variables
	viper.SetEnvPrefix("SHMOCKER")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// runBuildCommand handles the build command execution
func runBuildCommand(cmd *cobra.Command, args []string) error {
	buildPath := args[0]

	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Parse command flags
	buildReq, err := parseBuildFlags(cmd, buildPath, cfg)
	if err != nil {
		return fmt.Errorf("failed to parse build flags: %w", err)
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nBuild interrupted by user")
		cancel()
	}()

	// Execute build
	return executeBuild(ctx, buildReq, cmd)
}

// loadConfiguration loads the application configuration
func loadConfiguration() (*config.Config, error) {
	configPath := viper.GetString("config")
	// Don't specify a default path, let config.Load handle it
	return config.Load(configPath)
}

// parseBuildFlags parses command-line flags into a BuildRequest
func parseBuildFlags(cmd *cobra.Command, buildPath string, cfg *config.Config) (*builder.BuildRequest, error) {
	// Parse Dockerfile path
	dockerfilePath, _ := cmd.Flags().GetString("file")
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.Join(buildPath, dockerfilePath)
	}

	// Parse Dockerfile
	parser := dockerfile.New()
	ast, err := parser.ParseFile(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Dockerfile %s: %w", dockerfilePath, err)
	}

	// Validate Dockerfile
	if err := parser.Validate(ast); err != nil {
		return nil, fmt.Errorf("Dockerfile validation failed: %w", err)
	}

	// Parse build arguments
	buildArgs := make(map[string]string)
	buildArgSlice, _ := cmd.Flags().GetStringSlice("build-arg")
	for _, arg := range buildArgSlice {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			buildArgs[parts[0]] = parts[1]
		} else {
			// Get from environment
			buildArgs[parts[0]] = os.Getenv(parts[0])
		}
	}

	// Parse labels
	labels := make(map[string]string)
	labelSlice, _ := cmd.Flags().GetStringSlice("label")
	for _, label := range labelSlice {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}

	// Parse platforms
	var platforms []builder.Platform
	platformSlice, _ := cmd.Flags().GetStringSlice("platform")
	if len(platformSlice) == 0 && cfg.DefaultPlatform != "" {
		platformSlice = []string{cfg.DefaultPlatform}
	}
	for _, p := range platformSlice {
		platform, err := parsePlatform(p)
		if err != nil {
			return nil, fmt.Errorf("invalid platform %s: %w", p, err)
		}
		platforms = append(platforms, platform)
	}

	// Parse tags
	tags, _ := cmd.Flags().GetStringSlice("tag")

	// Parse target
	target, _ := cmd.Flags().GetString("target")

	// Parse cache settings
	cacheFrom, _ := cmd.Flags().GetStringSlice("cache-from")
	cacheTo, _ := cmd.Flags().GetStringSlice("cache-to")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	pull, _ := cmd.Flags().GetBool("pull")

	// Parse output configuration
	outputStr, _ := cmd.Flags().GetString("output")
	var outputConfig *builder.OutputConfig
	if outputStr != "" {
		var err error
		outputConfig, err = parseOutputConfig(outputStr)
		if err != nil {
			return nil, fmt.Errorf("invalid output configuration: %w", err)
		}
	}

	// Parse security features
	generateSBOM, _ := cmd.Flags().GetBool("sbom")
	signImage, _ := cmd.Flags().GetBool("sign")

	return &builder.BuildRequest{
		Context: builder.BuildContext{
			Type:         builder.ContextTypeLocal,
			Source:       buildPath,
			DockerIgnore: true,
		},
		Dockerfile:   ast,
		Tags:         tags,
		Target:       target,
		Platforms:    platforms,
		BuildArgs:    buildArgs,
		Labels:       labels,
		CacheFrom:    parseCacheImports(cacheFrom),
		CacheTo:      parseCacheExports(cacheTo),
		NoCache:      noCache,
		Output:       outputConfig,
		GenerateSBOM: generateSBOM,
		SignImage:    signImage,
		Pull:         pull,
	}, nil
}

// executeBuild executes the actual build process
func executeBuild(ctx context.Context, req *builder.BuildRequest, cmd *cobra.Command) error {
	// Create builder
	builderOpts := &builder.BuilderOptions{
		Root:     filepath.Join(os.TempDir(), "shmocker"),
		DataRoot: filepath.Join(os.TempDir(), "shmocker-data"),
		Debug:    verbose,
	}

	b, err := builder.New(ctx, builderOpts)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}
	defer b.Close()

	// Check if quiet mode
	quiet, _ := cmd.Flags().GetBool("quiet")
	progressType, _ := cmd.Flags().GetString("progress")

	if quiet {
		// Execute build without progress
		result, err := b.Build(ctx, req)
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		// Print only image ID in quiet mode
		fmt.Println(result.ImageID)
		return nil
	}

	// Execute build with progress
	progressChan := make(chan *builder.ProgressEvent, 100)
	done := make(chan struct{})

	// Start progress reporting goroutine
	go func() {
		defer close(done)
		reportProgress(progressChan, progressType)
	}()

	result, err := b.BuildWithProgress(ctx, req, progressChan)
	close(progressChan)
	<-done // Wait for progress reporting to finish

	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Print build results
	fmt.Printf("\nBuild completed successfully in %s\n", result.BuildTime)
	if result.ImageID != "" {
		fmt.Printf("Image ID: %s\n", result.ImageID)
	}
	if result.ImageDigest != "" {
		fmt.Printf("Image Digest: %s\n", result.ImageDigest)
	}

	return nil
}

// reportProgress handles progress reporting based on the specified format
func reportProgress(progressChan <-chan *builder.ProgressEvent, progressType string) {
	for event := range progressChan {
		switch progressType {
		case "json":
			// TODO: Implement JSON progress output
			fmt.Printf("{\"id\":\"%s\",\"status\":\"%s\"}\n", event.ID, event.Status)
		case "plain":
			fmt.Printf("[%s] %s\n", event.ID, event.Name)
		default: // auto, tty
			if event.Error != "" {
				fmt.Fprintf(os.Stderr, "ERROR [%s]: %s\n", event.ID, event.Error)
			} else {
				fmt.Printf("[%s] %s\n", event.ID, event.Name)
			}
		}
	}
}

// Helper functions for parsing

func parsePlatform(platformStr string) (builder.Platform, error) {
	parts := strings.Split(platformStr, "/")
	if len(parts) < 2 {
		return builder.Platform{}, fmt.Errorf("platform must be in format os/arch[/variant]")
	}

	platform := builder.Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}

	if len(parts) > 2 {
		platform.Variant = parts[2]
	}

	return platform, nil
}

func parseOutputConfig(outputStr string) (*builder.OutputConfig, error) {
	// Parse output string: type=local,dest=./output
	opts := make(map[string]string)
	parts := strings.Split(outputStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			opts[kv[0]] = kv[1]
		}
	}

	outputType := opts["type"]
	switch outputType {
	case "local":
		return &builder.OutputConfig{
			Type:        builder.OutputTypeLocal,
			Destination: opts["dest"],
		}, nil
	case "tar":
		return &builder.OutputConfig{
			Type:        builder.OutputTypeTar,
			Destination: opts["dest"],
		}, nil
	case "registry":
		return &builder.OutputConfig{
			Type:        builder.OutputTypeRegistry,
			Destination: opts["dest"],
			Push:        true,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported output type: %s", outputType)
	}
}

func parseCacheImports(cacheFrom []string) []*builder.CacheImport {
	var imports []*builder.CacheImport
	for _, cache := range cacheFrom {
		// Simple implementation - assume registry cache
		imports = append(imports, &builder.CacheImport{
			Type: "registry",
			Ref:  cache,
		})
	}
	return imports
}

func parseCacheExports(cacheTo []string) []*builder.CacheExport {
	var exports []*builder.CacheExport
	for _, cache := range cacheTo {
		// Simple implementation - assume registry cache
		exports = append(exports, &builder.CacheExport{
			Type: "registry",
			Ref:  cache,
		})
	}
	return exports
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
