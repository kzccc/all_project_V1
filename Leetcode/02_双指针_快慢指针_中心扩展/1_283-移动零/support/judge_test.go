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
			inputPath := filepath.Join("cases", base+".in")
			outputPath := filepath.Join("cases", base+".out")

			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			expected, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("read expected output: %v", err)
			}

			tmpDir := t.TempDir()
			copyFile(t, filepath.Join("..", "answer.go"), filepath.Join(tmpDir, "answer.go"))
			copyFile(t, "runner.go", filepath.Join(tmpDir, "runner.go"))
			copyFile(t, "go.mod", filepath.Join(tmpDir, "go.mod"))

			cmd := exec.Command("go", "run", "answer.go", "runner.go")
			cmd.Dir = tmpDir
			cmd.Stdin = bytes.NewReader(input)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				t.Fatalf("run failed: %v, stderr: %s", err, strings.TrimSpace(stderr.String()))
			}

			got := strings.TrimSpace(stdout.String())
			want := strings.TrimSpace(string(expected))
			if got != want {
				t.Fatalf("case %s failed\nexpected: %s\ngot: %s", base, want, got)
			}
		})
	}

	if !found {
		t.Fatal("no test cases found")
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
