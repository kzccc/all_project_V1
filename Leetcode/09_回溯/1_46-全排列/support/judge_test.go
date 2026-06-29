package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCases(t *testing.T) {
	entries, err := os.ReadDir("cases")
	if err != nil {
		t.Fatalf("read cases dir: %v", err)
	}
	found := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".in") {
			continue
		}
		found = true
		base := strings.TrimSuffix(entry.Name(), ".in")
		t.Run(base, func(t *testing.T) {
			input := mustRead(t, filepath.Join("cases", base+".in"))
			expected := strings.TrimSpace(string(mustRead(t, filepath.Join("cases", base+".out"))))
			tmpDir := t.TempDir()
			copyFile(t, filepath.Join("..", "answer.go"), filepath.Join(tmpDir, "answer.go"))
			copyFile(t, "runner.go", filepath.Join(tmpDir, "runner.go"))
			copyFile(t, "go.mod", filepath.Join(tmpDir, "go.mod"))
			cmd := exec.Command("go", "run", "answer.go", "runner.go")
			cmd.Dir = tmpDir
			cmd.Stdin = bytes.NewReader(input)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run failed: %v, stderr: %s", err, strings.TrimSpace(stderr.String()))
			}
			got := strings.TrimSpace(stdout.String())
			if got != expected {
				t.Fatalf("case %s failed\nexpected:\n%s\n\ngot:\n%s", base, expected, got)
			}
		})
	}
	if !found {
		t.Fatal("no test cases found")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data := mustRead(t, src)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
