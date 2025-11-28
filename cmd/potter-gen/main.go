package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		runInit()
	case "generate":
		runGenerate()
	case "update":
		runUpdate()
	case "sdk":
		runSDK()
	case "version":
		runVersion()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Potter Code Generator")
	fmt.Println()
	fmt.Println("Usage: potter-gen <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init       - Initialize a new project")
	fmt.Println("  generate   - Generate code from proto")
	fmt.Println("  update     - Update existing code")
	fmt.Println("  sdk        - Generate SDK")
	fmt.Println("  version    - Show version")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --proto    - Path to proto file (required)")
	fmt.Println("  --output   - Output directory (default: current dir)")
	fmt.Println("  --module   - Go module path (required for init)")
	fmt.Println("  --overwrite - Overwrite existing files")
	fmt.Println("  --interactive - Interactive mode for update")
	fmt.Println("  --sdk-only - Generate only SDK")
	fmt.Println("  --no-backup - Don't create backup on update")
}

