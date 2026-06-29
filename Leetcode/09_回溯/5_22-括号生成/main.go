package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		return
	}
	ans := generateParenthesisReference(n)
	fmt.Print(formatStrings(ans))
}

func generateParenthesisReference(n int) []string {
	path := make([]byte, 0, 2*n)
	ans := make([]string, 0)
	var dfs func(int, int)
	dfs = func(open, close int) {
		if len(path) == 2*n {
			ans = append(ans, string(append([]byte(nil), path...)))
			return
		}
		if open < n {
			path = append(path, '(')
			dfs(open+1, close)
			path = path[:len(path)-1]
		}
		if close < open {
			path = append(path, ')')
			dfs(open, close+1)
			path = path[:len(path)-1]
		}
	}
	dfs(0, 0)
	return ans
}

func formatStrings(data []string) string {
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, fmt.Sprintf("%d", len(data)))
	lines = append(lines, data...)
	return strings.Join(lines, "\n") + "\n"
}
