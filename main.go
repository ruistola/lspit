package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	filePath := os.Args[2]

	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving file path: %v\n", err)
		os.Exit(1)
	}

	// Verify file exists
	if _, err := os.Stat(absPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", absPath)
		os.Exit(1)
	}

	// Find workspace root (git repository root)
	workspaceRoot, err := findGitRoot(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding workspace root: %v\n", err)
		os.Exit(1)
	}

	// Create LSP client
	client, err := NewLSPClient(workspaceRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating LSP client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Initialize the LSP server
	if err := client.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing LSP server: %v\n", err)
		os.Exit(1)
	}

	// Execute command
	switch command {
	case "hover", "type":
		if len(os.Args) < 5 {
			fmt.Fprintf(os.Stderr, "Usage: %s hover <file> <line> <column>\n", os.Args[0])
			os.Exit(1)
		}
		line, col, err := parsePosition(os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing position: %v\n", err)
			os.Exit(1)
		}
		if err := client.Hover(absPath, line, col); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting hover info: %v\n", err)
			os.Exit(1)
		}

	case "definition", "def":
		if len(os.Args) < 5 {
			fmt.Fprintf(os.Stderr, "Usage: %s definition <file> <line> <column>\n", os.Args[0])
			os.Exit(1)
		}
		line, col, err := parsePosition(os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing position: %v\n", err)
			os.Exit(1)
		}
		if err := client.Definition(absPath, line, col); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting definition: %v\n", err)
			os.Exit(1)
		}

	case "references", "refs":
		if len(os.Args) < 5 {
			fmt.Fprintf(os.Stderr, "Usage: %s references <file> <line> <column>\n", os.Args[0])
			os.Exit(1)
		}
		line, col, err := parsePosition(os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing position: %v\n", err)
			os.Exit(1)
		}
		if err := client.References(absPath, line, col); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting references: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: mvp-lsp-client <command> <file> [args...]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  hover <file> <line> <column>      Get type information at position")
	fmt.Println("  definition <file> <line> <column> Find definition of symbol")
	fmt.Println("  references <file> <line> <column> Find all references to symbol")
	fmt.Println()
	fmt.Println("Line and column numbers are 1-indexed (matching editor display)")
}

func parsePosition(lineStr, colStr string) (int, int, error) {
	line, err := strconv.Atoi(lineStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid line number: %s", lineStr)
	}
	col, err := strconv.Atoi(colStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid column number: %s", colStr)
	}
	if line < 1 || col < 1 {
		return 0, 0, fmt.Errorf("line and column must be >= 1")
	}
	return line, col, nil
}
