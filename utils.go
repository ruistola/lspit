package main

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// findGitRoot finds the git repository root containing the given path
func findGitRoot(path string) (string, error) {
	// Get directory if path is a file
	dir := path
	if !isDir(dir) {
		dir = filepath.Dir(path)
	}

	// Run git rev-parse to find root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		// If not in a git repo, use the directory itself
		return dir, nil
	}

	root := strings.TrimSpace(string(output))
	return root, nil
}

func isDir(path string) bool {
	// Simple check without stat - if it has an extension, assume file
	return filepath.Ext(path) == ""
}
