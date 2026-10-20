package secscan

import (
	"strings"
	"testing"
)

func TestScanFileChecksumFirstLineNoPanic(t *testing.T) {
	// Regression (kern e2e, SliceManagerDeps): a 64-hex checksum on the
	// FIRST line of a file made isDocumentedChecksum compute src[:-1] and
	// panic with "slice bounds out of range [:-1]".
	src := []byte(strings.Repeat("ab", 32) + "  release.tar.gz\n")
	findings := ScanFile("docs/checksums.txt", src) // must not panic
	for _, f := range findings {
		if f.Rule != "hardcoded-secret" && f.Rule != "" {
			t.Errorf("unexpected rule for first-line checksum: %+v", f)
		}
	}
	// Sanity: an empty file stays clean.
	if got := ScanFile("docs/empty.txt", nil); len(got) != 0 {
		t.Errorf("expected no findings for empty file, got %+v", got)
	}
}

func TestScanFileFindsHardcodedSecret(t *testing.T) {
	src := []byte(`package main

import "os"

const apiKey = "sk-abcdefghijklmnopqrstuvwxyz1234567890"

func main() {
	os.Setenv("K", apiKey)
}
`)
	findings := ScanFile("config.go", src)
	if len(findings) == 0 {
		t.Fatal("expected findings for hardcoded secret")
	}
	hit := false
	for _, f := range findings {
		if f.Rule == "hardcoded-secret" && f.Line == 5 {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("expected hardcoded-secret on line 5, got %+v", findings)
	}
}

func TestScanFileDynamicSQL(t *testing.T) {
	src := []byte(`func query(db *sql.DB, id string) {
	rows, _ := db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = %s", id))
	rows2, _ := db.Query("SELECT * FROM users WHERE id = " + id)
	_ = rows
	_ = rows2
}
`)
	findings := ScanFile("db.go", src)
	if len(findings) == 0 {
		t.Fatal("expected sql-injection findings")
	}
	for _, f := range findings {
		if f.Rule != "sql-injection" {
			t.Fatalf("expected only sql-injection, got %+v", f)
		}
	}
}

func TestScanFileFalsePositiveFilters(t *testing.T) {
	src := []byte(`func check(db *sql.DB, table, user string) {
	rows, _ := db.Query("PRAGMA table_info(" + table + ")")   // const table: skipped
	rows2, _ := db.Query("PRAGMA table_info(" + user + ")")   // same shape: skipped by design
	rows3, _ := db.Query("PRAGMA table_info(" + getTable() + ")") // call: still flagged
	rows4, _ := db.Query("SELECT * FROM users WHERE id = " + user) // real SQL: flagged
	_ = rows
	_ = rows2
	_ = rows3
	_ = rows4
}
`)
	findings := ScanFile("db.go", src)
	var flagged []string
	for _, f := range findings {
		if f.Rule == "sql-injection" {
			flagged = append(flagged, f.Snippet)
		}
	}
	if len(flagged) != 2 {
		t.Fatalf("expected 2 sql-injection findings (call + real SQL), got %d: %v", len(flagged), flagged)
	}
	for _, s := range flagged {
		if !strings.Contains(s, "getTable()") && !strings.Contains(s, "WHERE id") {
			t.Fatalf("unexpected finding: %s", s)
		}
	}
}

func TestScanFileSelfRegexNotFlagged(t *testing.T) {
	src := []byte(`var re = regexp.MustCompile("(?i)\\b(?:md5\\.(?:New|Sum)|sha1\\.(?:New|Sum))\\b")
`)
	findings := ScanFile("detector.go", src)
	for _, f := range findings {
		if f.Rule == "weak-crypto" || f.Rule == "insecure-random" {
			t.Fatalf("detector regex flagged itself: %+v", f)
		}
	}
}

func TestScanFileCommandInjection(t *testing.T) {
	src := []byte(`func run(name string) {
	out, _ := exec.Command("sh", "-c", "cat "+name).Output()
	_ = out
}
`)
	findings := ScanFile("exec.go", src)
	if len(findings) == 0 {
		t.Fatal("expected command-injection finding")
	}
	found := false
	for _, f := range findings {
		if f.Rule == "command-injection" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected command-injection finding, got %+v", findings)
	}
}

func TestScanFileWeakCryptoAndRandom(t *testing.T) {
	src := []byte(`package main

func hash(s string) string {
	h := md5.Sum([]byte(s)) // deprecated crypto
	return fmt.Sprintf("%x", h)
}

func nonce() int {
	return rand.Intn(1e9) // security-relevant randomness
}
`)
	findings := ScanFile("crypto.go", src)
	if len(findings) == 0 {
		t.Fatal("expected weak-crypto/insecure-random findings")
	}
	rules := map[string]bool{}
	for _, f := range findings {
		rules[f.Rule] = true
	}
	if !rules["weak-crypto"] {
		t.Errorf("expected weak-crypto finding, got %+v", findings)
	}
	if !rules["insecure-random"] {
		t.Errorf("expected insecure-random finding (security keyword nearby), got %+v", findings)
	}
}

func TestScanFileUnsafeDeserialization(t *testing.T) {
	src := []byte(`func decode(data []byte) error {
	return json.Unmarshal(data, &map[string]interface{}{})
}
`)
	findings := ScanFile("decode.go", src)
	if len(findings) == 0 {
		t.Fatal("expected unsafe-deserialization finding")
	}
	for _, f := range findings {
		if f.Rule != "unsafe-deserialization" {
			t.Fatalf("expected only unsafe-deserialization, got %+v", f)
		}
	}
}

func TestScanFileCleanFile(t *testing.T) {
	src := []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello, world")
}
`)
	findings := ScanFile("main.go", src)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for clean file, got %+v", findings)
	}
}

func TestScanFileUnsafeHTTP(t *testing.T) {
	src := []byte(`func call(url string) {
	resp, _ := http.Get(url) // variable URL reaching an HTTP sink
	_ = resp
}
`)
	findings := ScanFile("http.go", src)
	if len(findings) == 0 {
		t.Fatal("expected unsafe-http finding")
	}
	for _, f := range findings {
		if f.Rule != "unsafe-http" {
			t.Fatalf("expected only unsafe-http, got %+v", f)
		}
	}
}

func TestScanFileUnsafeExec(t *testing.T) {
	src := []byte(`func run(bin, arg string) {
	out, _ := exec.Command(bin, arg).Output()
	_ = out
}
`)
	findings := ScanFile("exec.go", src)
	if len(findings) == 0 {
		t.Fatal("expected unsafe-exec finding")
	}
	for _, f := range findings {
		if f.Rule != "unsafe-exec" {
			t.Fatalf("expected only unsafe-exec, got %+v", f)
		}
	}
}

func TestScanFileWellKnownGitHashAllowlisted(t *testing.T) {
	src := []byte(`package main

// the empty tree and null SHA are git sentinels, not secrets
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
const nullSHA = "0000000000000000000000000000000000000000"
`)
	findings := ScanFile("git.go", src)
	for _, f := range findings {
		if f.Rule == "hardcoded-secret" {
			t.Fatalf("well-known git hash flagged as secret: %+v", f)
		}
	}
}

func TestScanFilesSkipsTestFiles(t *testing.T) {
	// Test fixtures routinely hold fake secrets; they must not be findings.
	secret := []byte(`const key = "sk-abcdefghijklmnopqrstuvwxyz1234567890"`)
	if got := ScanFiles([]ScanTarget{{Rel: "auth_test.go", Src: secret}}); len(got) != 0 {
		t.Fatalf("test file fixture must be skipped, got %+v", got)
	}
	// A real source file with the same content IS flagged.
	if got := ScanFiles([]ScanTarget{{Rel: "auth.go", Src: secret}}); len(got) == 0 {
		t.Fatal("source file with secret must be flagged")
	}
}

func TestScanFilesSkipsMinifiedJS(t *testing.T) {
	// Minified bundles pack libraries into single huge lines that trip the
	// detectors with bogus findings.
	big := strings.Repeat("x", 5000)
	src := []byte(`(function(){var a="` + big + `";var b="sk-abcdefghijklmnopqrstuvwxyz1234567890";})();`)
	if got := ScanFiles([]ScanTarget{{Rel: "static/bundle.min.js", Src: src}}); len(got) != 0 {
		t.Fatalf("minified JS must be skipped, got %+v", got)
	}
	// A normal JS file with the same secret is flagged.
	src2 := []byte("const key = \"sk-abcdefghijklmnopqrstuvwxyz1234567890\";\n")
	if got := ScanFiles([]ScanTarget{{Rel: "app.js", Src: src2}}); len(got) == 0 {
		t.Fatal("non-minified JS with secret must be flagged")
	}
}

func TestScanFilesSkipsOversizedFiles(t *testing.T) {
	src := make([]byte, 10*1024*1024+1)
	src[0] = 'a'
	if got := ScanFiles([]ScanTarget{{Rel: "huge.go", Src: src}}); len(got) != 0 {
		t.Fatalf("oversized file must be skipped, got %+v", got)
	}
}

func TestScanFilesConfigAndPythonPasses(t *testing.T) {
	// Config files get the config-aware pass.
	cfg := []byte("spring.datasource.password=admin\n")
	found := false
	for _, f := range ScanFiles([]ScanTarget{{Rel: "application.properties", Src: cfg}}) {
		if f.Rule == "hardcoded-config-cred" {
			found = true
		}
	}
	if !found {
		t.Error("expected hardcoded-config-cred finding in config file")
	}
	// Python files get the Python-aware pass.
	py := []byte("import subprocess\nsubprocess.run(\"ls -la\", shell=True)\n")
	found = false
	for _, f := range ScanFiles([]ScanTarget{{Rel: "run.py", Src: py}}) {
		if f.Rule == "py-subprocess-shell" {
			found = true
		}
	}
	if !found {
		t.Error("expected py-subprocess-shell finding in python file")
	}
}

func TestScanConfigFilePlaceholderBug(t *testing.T) {
	// $VAR without braces is a Spring placeholder bug.
	src := []byte("db.url=jdbc:mysql://$DB_HOST:$DB_PORT/app\n")
	found := false
	for _, f := range ScanConfigFile("application.properties", src) {
		if f.Rule == "placeholder-bug" {
			found = true
		}
	}
	if !found {
		t.Error("expected placeholder-bug finding")
	}
}

func TestFilterBySeverityAndFiles(t *testing.T) {
	fs := []Finding{
		{File: "a.go", Line: 1, Rule: "x", Severity: "error"},
		{File: "b.go", Line: 2, Rule: "y", Severity: "warning"},
		{File: "c.go", Line: 3, Rule: "z", Severity: "info"},
	}
	if got := FilterBySeverity(fs, []string{"error"}); len(got) != 1 || got[0].File != "a.go" {
		t.Fatalf("FilterBySeverity error: %+v", got)
	}
	if got := FilterBySeverity(fs, nil); len(got) != 3 {
		t.Fatalf("FilterBySeverity empty allow must keep all: %+v", got)
	}
	if got := FilterByFiles(fs, []string{"b.go"}); len(got) != 1 || got[0].File != "b.go" {
		t.Fatalf("FilterByFiles: %+v", got)
	}
	if got := FilterByFiles(fs, nil); got != nil {
		t.Fatalf("FilterByFiles nil files must return nil, got %+v", got)
	}
}

func TestRenderAndCounts(t *testing.T) {
	fs := []Finding{
		{File: "a.go", Line: 1, Rule: "x", Severity: "error", Message: "m1"},
		{File: "a.go", Line: 2, Rule: "y", Severity: "warning", Message: "m2"},
		{File: "a.go", Line: 3, Rule: "z", Severity: "info", Message: "m3"},
	}
	out := Render(fs, 2)
	if !strings.Contains(out, "... and 1 more findings") {
		t.Fatalf("Render max truncation missing: %q", out)
	}
	c := Counts(fs)
	if c["error"] != 1 || c["warning"] != 1 || c["info"] != 1 {
		t.Fatalf("Counts: %+v", c)
	}
}