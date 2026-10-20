package secscan

import "testing"

func TestScanFileSkipsCodeExpressionSecrets(t *testing.T) {
	// Struct literals and stdlib conversions are construction, not leaks.
	src := []byte(`package main

import (
	"crypto/ed25519"
)

func load(priv []byte) ed25519.PrivateKey {
	return ed25519.PrivateKey(priv)
}

type C struct {
	Key string
}

func use() C {
	return C{Key: "sk-abcdefghijklmnopqrstuvwxyz1234567890"}
}
`)
	findings := ScanFile("keys.go", src)
	// The struct literal IS a hardcoded secret (quoted value); the
	// ed25519.PrivateKey(priv) conversion must NOT be flagged as a secret.
	flaggedConversion := false
	secretCount := 0
	for _, f := range findings {
		if f.Rule != "hardcoded-secret" {
			continue
		}
		secretCount++
		if f.Snippet != "" && f.Line == 4 {
			flaggedConversion = true
		}
	}
	if flaggedConversion {
		t.Fatalf("code conversion flagged as secret: %+v", findings)
	}
	// The struct literal on the quoted line must still be flagged.
	if secretCount == 0 {
		t.Fatalf("expected the struct-literal secret to be flagged: %+v", findings)
	}
}

func TestScanFileDedupesSameLineSameRule(t *testing.T) {
	// Two matches of one rule on one line (two md5.Sum calls in one
	// Sprintf) are a single finding, not two.
	src := []byte(`package main

import "fmt"

func hashes(a, b string) string {
	return fmt.Sprintf("%x %x", md5.Sum([]byte(a)), md5.Sum([]byte(b)))
}
`)
	findings := ScanFile("hash.go", src)
	count := 0
	for _, f := range findings {
		if f.Rule == "weak-crypto" && f.Line == 6 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 weak-crypto finding on line 6, got %d: %+v", count, findings)
	}
}

func TestScanFileSkipsCanonicalExampleSecrets(t *testing.T) {
	// AWS docs example key pair and the canonical fake token are
	// allowlisted by construction.
	src := []byte(`const accessKey = "AKIAIOSFODNN7EXAMPLE"
const secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
const fake = "sk-abc123"
`)
	findings := ScanFile("examples.go", src)
	for _, f := range findings {
		if f.Rule == "hardcoded-secret" {
			t.Fatalf("canonical example secret flagged: %+v", f)
		}
	}
}

func TestScanFileSkipsFixtureDirFiles(t *testing.T) {
	// Files under /fixtures/ are test fixtures (isTestFile), so their fake
	// secrets must not be findings in the diff-gate path either.
	src := []byte(`const key = "sk-abcdefghijklmnopqrstuvwxyz1234567890"`)
	if got := ScanFiles([]ScanTarget{{Rel: "test/fixtures/creds.go", Src: src}}); len(got) != 0 {
		t.Fatalf("fixture dir file must be skipped, got %+v", got)
	}
}
