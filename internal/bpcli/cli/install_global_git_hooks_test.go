package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnv builds a hermetic environment: a throwaway HOME (so core.hooksPath
// and the bypass audit never touch the real machine), no kern on PATH.
func gitEnv(home string, extra ...string) []string {
	env := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "KERN_") ||
			strings.HasPrefix(e, "GIT_CONFIG") || strings.HasPrefix(e, "XDG_CONFIG_HOME=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env,
		"HOME="+home,
		"GIT_CONFIG_GLOBAL="+filepath.Join(home, ".gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"KERN_BINARY="+filepath.Join(home, "no-such-kern"),
	)
	return append(env, extra...)
}

func hookGit(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustHookGit(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	out, err := hookGit(t, dir, env, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func installHooksInto(t *testing.T, home string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	if err := InstallGlobalGitHooks(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallGlobalGitHooksWritesEveryHook(t *testing.T) {
	home := t.TempDir()
	installHooksInto(t, home)
	dir := filepath.Join(home, ".kern", "git-hooks")
	names := append([]string{"pre-commit", "post-commit", "pre-push"}, globalPassthroughHooks...)
	for _, n := range names {
		fi, err := os.Stat(filepath.Join(dir, n))
		if err != nil {
			t.Errorf("hook %s not installed: %v", n, err)
			continue
		}
		if fi.Mode()&0o111 == 0 {
			t.Errorf("hook %s is not executable", n)
		}
	}
	if ok, why := IsGlobalGitHooksInstalled(); !ok {
		t.Errorf("IsGlobalGitHooksInstalled = false: %s", why)
	}
}

func TestGlobalHooksFailOpenWithoutKernAndChainRepoHooks(t *testing.T) {
	home := t.TempDir()
	installHooksInto(t, home)
	env := gitEnv(home)

	repo := t.TempDir()
	mustHookGit(t, repo, env, "init", "-q", "-b", "main")
	hooks := filepath.Join(repo, ".git", "hooks")
	marker := filepath.Join(repo, "commit-msg-ran")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(hooks, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("commit-msg", "echo ran > '"+marker+"'\n")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustHookGit(t, repo, env, "add", "a.txt")

	// kern is missing (KERN_BINARY points nowhere): commit must still succeed,
	// and the repo's own commit-msg hook must still run through the shim.
	if out, err := hookGit(t, repo, env, "commit", "-q", "-m", "first"); err != nil {
		t.Fatalf("commit must fail open without kern: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("repo's own commit-msg hook was silently disabled by the global hooksPath")
	}

	// The repo's own pre-commit hook must still be able to block a commit.
	write("pre-commit", "echo 'repo rule says no' >&2\nexit 1\n")
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustHookGit(t, repo, env, "add", "b.txt")
	if out, err := hookGit(t, repo, env, "commit", "-q", "-m", "second"); err == nil {
		t.Fatalf("repo pre-commit hook was not chained; commit succeeded:\n%s", out)
	} else if !strings.Contains(out, "repo rule says no") {
		t.Errorf("repo hook output missing:\n%s", out)
	}
}

func TestGlobalPrePushBlocksForceAndDeleteOnProtectedBranch(t *testing.T) {
	home := t.TempDir()
	installHooksInto(t, home)
	env := gitEnv(home)

	remote := t.TempDir()
	mustHookGit(t, remote, env, "init", "-q", "--bare", "-b", "main")
	repo := t.TempDir()
	mustHookGit(t, repo, env, "init", "-q", "-b", "main")
	mustHookGit(t, repo, env, "remote", "add", "origin", remote)
	commit := func(file string) {
		if err := os.WriteFile(filepath.Join(repo, file), []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		mustHookGit(t, repo, env, "add", file)
		mustHookGit(t, repo, env, "commit", "-q", "-m", file)
	}
	commit("one")
	commit("two")

	if out, err := hookGit(t, repo, env, "push", "-q", "origin", "main"); err != nil {
		t.Fatalf("a normal push must pass: %v\n%s", err, out)
	}

	// Rewrite history, then try to force-push it.
	mustHookGit(t, repo, env, "reset", "-q", "--hard", "HEAD~1")
	commit("three")
	out, err := hookGit(t, repo, env, "push", "-q", "--force", "origin", "main")
	if err == nil || !strings.Contains(out, "non-fast-forward") {
		t.Fatalf("force push to main must be blocked by the kern hook (err=%v):\n%s", err, out)
	}

	// KERN_DISABLED is not a silent off-switch for the git hooks: only the
	// audited KERN_BYPASS gets past the push guard.
	disabled := gitEnv(home, "KERN_DISABLED=1")
	if out, err := hookGit(t, repo, disabled, "push", "-q", "--force", "origin", "main"); err == nil {
		t.Fatalf("KERN_DISABLED=1 must not silently bypass the push guard:\n%s", out)
	}

	if out, err := hookGit(t, repo, env, "push", "-q", "origin", ":main"); err == nil || !strings.Contains(out, "delete protected branch") {
		t.Fatalf("deleting main must be blocked (err=%v):\n%s", err, out)
	}

	// Non-protected branches are never touched by the guard.
	mustHookGit(t, repo, env, "checkout", "-q", "-b", "feature")
	if out, err := hookGit(t, repo, env, "push", "-q", "--force", "origin", "feature"); err != nil {
		t.Fatalf("force push to an unprotected branch must pass: %v\n%s", err, out)
	}

	// Audited break-glass.
	bypass := gitEnv(home, "KERN_BYPASS=1", "KERN_BYPASS_REASON=hotfix")
	mustHookGit(t, repo, bypass, "checkout", "-q", "main")
	if out, err := hookGit(t, repo, bypass, "push", "-q", "--force", "origin", "main"); err != nil {
		t.Fatalf("KERN_BYPASS=1 must allow the push: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(home, ".kern", "audit", "bypass.jsonl"))
	if err != nil || !strings.Contains(string(data), `"hook":"pre-push"`) {
		t.Errorf("bypassed push must be audited: err=%v data=%s", err, data)
	}
}

func TestGlobalPrePushProtectedBranchesOverride(t *testing.T) {
	home := t.TempDir()
	installHooksInto(t, home)
	env := gitEnv(home, "KERN_PROTECTED_BRANCHES=release")

	remote := t.TempDir()
	mustHookGit(t, remote, env, "init", "-q", "--bare", "-b", "main")
	repo := t.TempDir()
	mustHookGit(t, repo, env, "init", "-q", "-b", "main")
	mustHookGit(t, repo, env, "remote", "add", "origin", remote)
	for _, f := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(repo, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
		mustHookGit(t, repo, env, "add", f)
		mustHookGit(t, repo, env, "commit", "-q", "-m", f)
	}
	mustHookGit(t, repo, env, "push", "-q", "origin", "main")
	mustHookGit(t, repo, env, "reset", "-q", "--hard", "HEAD~1")
	if err := os.WriteFile(filepath.Join(repo, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustHookGit(t, repo, env, "add", "x")
	mustHookGit(t, repo, env, "commit", "-q", "-m", "x")
	if out, err := hookGit(t, repo, env, "push", "-q", "--force", "origin", "main"); err != nil {
		t.Fatalf("main is not protected when KERN_PROTECTED_BRANCHES=release: %v\n%s", err, out)
	}
}
