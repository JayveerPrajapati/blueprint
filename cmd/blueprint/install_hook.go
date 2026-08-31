package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// runInstallHook implements `blueprint install hook`.
// It writes a pre-commit hook script to .git/hooks/pre-commit that calls
// `blueprint check --staged --format=terminal`.
func runInstallHook(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(os.Stderr, "Usage: blueprint install hook\n\nInstall a pre-commit hook that runs `blueprint check --staged`.")
		return 0
	}

	// Find the git directory.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "blueprint: cannot determine working directory: %v\n", err)
		return 2
	}
	gitDir, err := findGitDir(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "blueprint: %v\n", err)
		return 2
	}

	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")

	// Check if a hook already exists and is NOT our hook.
	if existing, err := os.ReadFile(hookPath); err == nil {
		if !isBlueprintHook(existing) {
			fmt.Fprintf(os.Stderr, "blueprint: a pre-commit hook already exists at %s\n", hookPath)
			fmt.Fprintln(os.Stderr, "  To overwrite, remove it first: rm .git/hooks/pre-commit")
			fmt.Fprintln(os.Stderr, "  Then re-run: blueprint install hook")
			return 2
		}
	}

	hooksDir := filepath.Dir(hookPath)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "blueprint: cannot create hooks directory: %v\n", err)
		return 2
	}

	// Write the hook script. It is a thin adapter: it only invokes
	// `blueprint check --staged` and propagates its exit code (spec Phase 4,
	// Rule 3 — no command execution logic duplicated in the shell script).
	hookContent := "#!/bin/sh\n" +
		"# Blueprint pre-commit hook — thin adapter to `blueprint check --staged`\n" +
		"# Installed by `blueprint install hook`. To bypass: `git commit --no-verify`\n" +
		"# (this is documented, not a bug — see spec Rule 3)\n" +
		"exec blueprint check --staged --format=terminal\n"
	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "blueprint: cannot write hook: %v\n", err)
		return 2
	}

	if err := os.Chmod(hookPath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "blueprint: cannot chmod hook: %v\n", err)
		return 2
	}

	fmt.Printf("Installed pre-commit hook at %s\n", hookPath)
	fmt.Println("The hook runs `blueprint check --staged` on every commit.")
	fmt.Println("To bypass: git commit --no-verify (documented, not a bug)")
	return 0
}

// findGitDir walks up from start to find a .git directory or .git file
// (for worktrees).
func findGitDir(start string) (string, error) {
	dir := start
	for i := 0; i < 20; i++ {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				return gitPath, nil
			}
			// .git is a file (worktree) — read the gitdir pointer and
			// resolve the common git dir so the hook lands where git
			// actually reads it.
			gitDir, err := worktreeGitDir(gitPath)
			if err != nil {
				return "", fmt.Errorf("cannot read gitdir pointer %s: %v", gitPath, err)
			}
			return commonGitDir(gitDir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("not a git repository (no .git found walking up from %s)", start)
}

// worktreeGitDir reads the `gitdir: <path>` line from a .git file (a git
// worktree) and resolves it to an absolute path.
func worktreeGitDir(gitFile string) (string, error) {
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", err
	}
	line := bytes.TrimSpace(data)
	prefix := []byte("gitdir:")
	if !bytes.HasPrefix(line, prefix) {
		return "", fmt.Errorf("malformed .git file: missing gitdir line")
	}
	path := string(bytes.TrimSpace(line[len(prefix):]))
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(gitFile), path)
	}
	return path, nil
}

// commonGitDir returns the common git directory for a worktree gitdir.
// Hooks live in the common dir, not in .git/worktrees/<name>.
func commonGitDir(gitDir string) string {
	const marker = "/.git/worktrees/"
	if idx := indexOfBytes([]byte(gitDir), []byte(marker)); idx >= 0 {
		return gitDir[:idx] + "/.git"
	}
	return gitDir
}

// isBlueprintHook returns true if the existing hook content was installed by
// Blueprint (contains the Blueprint marker comment).
func isBlueprintHook(content []byte) bool {
	return bytes.Contains(content, []byte("Blueprint pre-commit hook"))
}

// indexOfBytes reports the index of the first instance of sep in s, or -1.
func indexOfBytes(s, sep []byte) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		match := true
		for j := 0; j < len(sep); j++ {
			if s[i+j] != sep[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
