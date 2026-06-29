package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Problem struct {
	Topic string
	ID    string
	Title string
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail("getwd: %v", err)
	}

	problems, err := parseHot100(filepath.Join(root, "LeetCodeHot100-面试最佳解法分类题单.md"))
	if err != nil {
		fail("parse hot100: %v", err)
	}
	existing, err := existingProblemIDs(root)
	if err != nil {
		fail("scan existing problems: %v", err)
	}

	created := 0
	skipped := 0
	for _, p := range problems {
		if existing[p.ID] {
			skipped++
			continue
		}
		dir := filepath.Join(root, p.Topic, fmt.Sprintf("%s-%s", p.ID, p.Title))
		if err := os.MkdirAll(filepath.Join(dir, "support", "cases"), 0755); err != nil {
			fail("mkdir %s: %v", dir, err)
		}
		if err := writeProblemPackage(dir, p); err != nil {
			fail("write package %s: %v", dir, err)
		}
		existing[p.ID] = true
		created++
	}

	fmt.Printf("parsed=%d created=%d skipped=%d\n", len(problems), created, skipped)
}

func parseHot100(path string) ([]Problem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	sectionRe := regexp.MustCompile("^##\\s+\\d+\\.\\s+(.+?)(?:（\\d+）)?$")
	problemRe := regexp.MustCompile("^- \\[ \\] `([0-9]+)\\. (.+)`$")

	var problems []Problem
	var topic string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			topic = normalizeTopic(m[1])
			continue
		}
		if m := problemRe.FindStringSubmatch(line); m != nil {
			if topic == "" {
				return nil, fmt.Errorf("problem before topic: %s", line)
			}
			problems = append(problems, Problem{
				Topic: topic,
				ID:    m[1],
				Title: sanitizeTitle(m[2]),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(problems) != 100 {
		return nil, fmt.Errorf("expected 100 problems, got %d", len(problems))
	}
	return problems, nil
}

func normalizeTopic(raw string) string {
	replacer := strings.NewReplacer(
		" / ", "_",
		" + ", "_",
		"：", "_",
		" ", "",
		"/", "_",
		"+", "_",
	)
	return replacer.Replace(raw)
}

func sanitizeTitle(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer(
		"/", "_",
		"?", "",
		"*", "",
		":", "：",
		"(", "（",
		")", "）",
	)
	return replacer.Replace(s)
}

func existingProblemIDs(root string) (map[string]bool, error) {
	ids := make(map[string]bool)
	re := regexp.MustCompile(`^(\d+)-`)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "support" || strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, "answer.go")); statErr == nil {
			if m := re.FindStringSubmatch(filepath.Base(path)); m != nil {
				ids[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func writeProblemPackage(dir string, p Problem) error {
	slug := fmt.Sprintf("lc%s", p.ID)
	if err := writeFile(filepath.Join(dir, "answer.go"), answerGo(p)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "main.go"), mainGo(p)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "题解.md"), explanationMD(p)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "例子推演过程.md"), walkthroughMD(p)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "go.mod"), fmt.Sprintf("module %s\n\ngo 1.22\n", slug)); err != nil {
		return err
	}
	supportDir := filepath.Join(dir, "support")
	if err := writeFile(filepath.Join(supportDir, "runner.go"), runnerGo(p)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "judge_test.go"), judgeTestGo()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "answer_stub_test.go"), "package main\n\n"); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "go.mod"), fmt.Sprintf("module %s_support\n\ngo 1.22\n", slug)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "题目要求.md"), requirementMD(p)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "cases", "sample1.in"), defaultCaseIn()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "cases", "sample1.out"), defaultCaseOut()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "cases", "sample2.in"), defaultCaseIn()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "cases", "sample2.out"), defaultCaseOut()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "cases", "edge1.in"), defaultCaseIn()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(supportDir, "cases", "edge1.out"), defaultCaseOut()); err != nil {
		return err
	}
	return nil
}

func answerGo(p Problem) string {
	return "package main\n\n// TODO: 在这里实现面试版核心解法。\n"
}

func mainGo(p Problem) string {
	return fmt.Sprintf(`package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	_ = in
	fmt.Println("TODO: %s. %s 的参考 ACM 实现待补充")
}
`, p.ID, p.Title)
}

func explanationMD(p Problem) string {
	return fmt.Sprintf("# %s. %s\n\n"+
		"## 题目概述\n\n"+
		"这道题属于 `%s` 分类。这里先保留统一训练包结构，后续再补充该题的完整题解。\n\n"+
		"## 面试最佳解法\n\n"+
		"待补充：\n\n"+
		"1. 这题在面试里最主流的解法路线。\n"+
		"2. 为什么它比其他可行写法更适合作为首选答案。\n\n"+
		"## 核心思路\n\n"+
		"待补充。\n\n"+
		"## 关键状态或不变量\n\n"+
		"待补充。\n\n"+
		"## 复杂度\n\n"+
		"- 时间复杂度：待补充\n"+
		"- 空间复杂度：待补充\n\n"+
		"## 容易写错的点\n\n"+
		"待补充。\n\n"+
		"## 备选方案\n\n"+
		"待补充。\n",
		p.ID, p.Title, p.Topic)
}

func walkthroughMD(p Problem) string {
	return fmt.Sprintf("# %s. %s 例子推演过程\n\n"+
		"这道题的统一训练包已经创建完成。\n\n"+
		"后续在补参考解时，这里会加入一个中等复杂度用例，并按步骤展示：\n\n"+
		"1. 输入如何被抽象成算法状态。\n"+
		"2. 每一步状态如何变化。\n"+
		"3. 为什么最终能得到正确答案。\n\n"+
		"当前先保留这个占位文档，确保目录结构完整一致。\n",
		p.ID, p.Title)
}

func requirementMD(p Problem) string {
	return fmt.Sprintf("# %s. %s\n\n"+
		"你需要在根目录 `answer.go` 中实现该题的核心函数。\n\n"+
		"当前状态：\n\n"+
		"- 训练包骨架已生成\n"+
		"- 参考 ACM 实现待补充\n"+
		"- 测试样例待按题意补全\n\n"+
		"只修改 `../answer.go`。\n\n"+
		"在 `support/` 目录下运行：\n\n"+
		"```bash\n"+
		"go test\n"+
		"```\n",
		p.ID, p.Title)
}

func runnerGo(p Problem) string {
	return fmt.Sprintf(`package main

import "fmt"

func main() {
	fmt.Println("TODO: %s. %s 的 runner 待补充")
}
`, p.ID, p.Title)
}

func judgeTestGo() string {
	return `package main

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
`
}

func defaultCaseIn() string {
	return "placeholder\n"
}

func defaultCaseOut() string {
	return "TODO\n"
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
