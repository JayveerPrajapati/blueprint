// Package main is the Blueprint CLI entry point.
package main

import (
	"fmt"
	"os"

	"github.com/JayveerPrajapati/blueprint/internal/bpcli/cli"
	blueprintversion "github.com/JayveerPrajapati/blueprint/internal/bpreceipt/version"
)

var version = blueprintversion.Version

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(0)
	}

	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case "version", "--version", "-v", "-version":
		fmt.Printf("blueprint %s\n", version)
	case "watch":
		os.Exit(runWatch(args))
	case "help", "--help", "-h":
		usage(os.Stdout)
	case "doctor":
		os.Exit(cli.RunDoctor(args))
	case "check":
		os.Exit(cli.RunCheck(args))
	case "fix":
		os.Exit(cli.RunFix(args))
	case "install":
		os.Exit(cli.RunInstall(args))
	case "ci":
		os.Exit(cli.RunCI(args))
	case "verify-receipt":
		os.Exit(cli.RunVerifyReceipt(args))
	case "metrics":
		os.Exit(cli.RunMetrics(args))
	case "graph":
		os.Exit(cli.RunGraph(args))
	case "request-approval":
		os.Exit(cli.RunRequestApproval(args))
	case "approve":
		os.Exit(cli.RunApprovalDecision("approve", args))
	case "reject":
		os.Exit(cli.RunApprovalDecision("reject", args))
	default:
		fmt.Fprintf(os.Stderr, "blueprint: unknown command %q\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, `blueprint — local change governance engine

Usage:
  blueprint <command> [flags]

Commands:
  version       Print the blueprint version
  doctor        Preflight environment/configuration diagnostic
  check         Validate staged changes against policy
  fix           Validate agent-proposed fixes in an isolated worktree
  install hook  Install a pre-commit hook that runs `+"`blueprint check --staged`"+`
  watch         Continuous advisory file-change watcher
  ci            CI/protected-branch enforcement (base vs head, JSON artifact)
  verify-receipt
                Verify a tamper-evident CI receipt (receipt + audit chain)
  metrics       Show local validation metrics (no cloud telemetry)
  graph         Print the change graph for the working tree
  request-approval
                Request human approval for a high-risk change (two-person rule)
  approve       Approve a pending approval request: approve <id> [--reason ...]
  reject        Reject a pending approval request: reject <id> [--reason ...]
  help          Show this help

Exit codes (spec Section 6):
  0   PASS
  1   policy violation / BLOCK (fix: ANY remaining finding — WARN or BLOCK — exits 1; the repair loop must iterate)
  2   tool/runtime/configuration/usage ERROR
  3   invalid Blueprint configuration
  4   unsupported operation or environment`)
}
