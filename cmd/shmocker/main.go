package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	Run: func(cmd *cobra.Command, args []string) {
		buildPath := args[0]
		fmt.Printf("Building image from path: %s\n", buildPath)
		
		// TODO: Implement actual build logic
		fmt.Println("Build functionality not yet implemented")
	},
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

	// Build command flags
	buildCmd.Flags().StringP("tag", "t", "", "tag for the built image")
	buildCmd.Flags().StringP("file", "f", "Dockerfile", "path to Dockerfile")
	buildCmd.Flags().Bool("no-cache", false, "do not use cache when building")
	buildCmd.Flags().Bool("pull", false, "always pull base images")
	buildCmd.Flags().StringSlice("build-arg", []string{}, "build arguments")
	buildCmd.Flags().StringSlice("label", []string{}, "image labels")
	buildCmd.Flags().String("platform", "", "target platform (e.g., linux/amd64)")
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}