package secscan

import (
	"strings"
	"testing"
)

func TestScanPythonFileSinks(t *testing.T) {
	src := []byte(`import os
import subprocess
import pickle
import yaml

def run(cmd):
    # shell=True variant
    subprocess.run(cmd, shell=True)

def load(path):
    with open(path) as f:
        data = yaml.load(f)

def unsafe(data):
    return pickle.loads(data)

def query(db, uid):
    return db.execute(f"SELECT * FROM users WHERE id = {uid}")

def go():
    os.system("ls -la")
`)
	findings := ScanPythonFile("app.py", src)
	rules := map[string]bool{}
	for _, f := range findings {
		rules[f.Rule] = true
	}
	for _, want := range []string{"py-subprocess-shell", "py-yaml-load", "py-pickle-load", "py-sql-format", "py-os-system"} {
		if !rules[want] {
			t.Errorf("missing %s rule; got %+v", want, rules)
		}
	}
}

func TestScanPythonFileSkipsCommentsAndDoc(t *testing.T) {
	src := []byte(`# subprocess.run("x", shell=True) — documented example
# yaml.load(path) — mentioned in a comment

def ok():
    # eval() inside a comment is not execution
    return 1
`)
	findings := ScanPythonFile("doc.py", src)
	if len(findings) != 0 {
		t.Fatalf("comment-only python must have no findings, got %+v", findings)
	}
}

func TestScanPythonFileMethodCallsNotGlobal(t *testing.T) {
	src := []byte(`class Model:
    def eval(self):
        return 1

model = Model()
model.eval()  # method call on an instance, not a global call
`)
	findings := ScanPythonFile("model.py", src)
	for _, f := range findings {
		if f.Rule == "py-eval" {
			t.Fatalf("method/def eval() flagged as global eval: %+v", f)
		}
	}
}

func TestScanPythonFileSnippetCapped(t *testing.T) {
	line := strings.Repeat("x", 200)
	src := []byte("eval(" + line + ")\n")
	findings := ScanPythonFile("a.py", src)
	if len(findings) == 0 {
		t.Fatal("expected py-eval finding")
	}
	if len(findings[0].Snippet) > 120 {
		t.Fatalf("snippet not capped at 120 chars: %q", findings[0].Snippet)
	}
}
