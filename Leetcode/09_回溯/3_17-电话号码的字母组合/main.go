package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var digits string
	if _, err := fmt.Fscan(in, &digits); err != nil {
		return
	}
	ans := letterCombinationsReference(digits)
	fmt.Print(formatStrings(ans))
}

func letterCombinationsReference(digits string) []string {
	if len(digits) == 0 {
		return []string{}
	}
	mapping := map[byte]string{
		'2': "abc",
		'3': "def",
		'4': "ghi",
		'5': "jkl",
		'6': "mno",
		'7': "pqrs",
		'8': "tuv",
		'9': "wxyz",
	}
	path := make([]byte, 0, len(digits))
	ans := make([]string, 0)
	var dfs func(int)
	dfs = func(i int) {
		if i == len(digits) {
			ans = append(ans, string(append([]byte(nil), path...)))
			return
		}
		letters := mapping[digits[i]]
		for j := 0; j < len(letters); j++ {
			path = append(path, letters[j])
			dfs(i + 1)
			path = path[:len(path)-1]
		}
	}
	dfs(0)
	return ans
}

func formatStrings(data []string) string {
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, fmt.Sprintf("%d", len(data)))
	lines = append(lines, data...)
	return strings.Join(lines, "\n") + "\n"
}
