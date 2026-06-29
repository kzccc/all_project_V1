package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var s string
	var m int
	if _, err := fmt.Fscan(in, &s, &m); err != nil {
		return
	}
	wordDict := make([]string, m)
	for i := 0; i < m; i++ {
		fmt.Fscan(in, &wordDict[i])
	}
	fmt.Println(wordBreakReference(s, wordDict))
}

func wordBreakReference(s string, wordDict []string) bool {
	set := make(map[string]struct{}, len(wordDict))
	maxLen := 0
	for _, word := range wordDict {
		set[word] = struct{}{}
		if len(word) > maxLen {
			maxLen = len(word)
		}
	}

	dp := make([]bool, len(s)+1)
	dp[0] = true
	for i := 1; i <= len(s); i++ {
		start := i - maxLen
		if start < 0 {
			start = 0
		}
		for j := start; j < i; j++ {
			if dp[j] {
				if _, ok := set[s[j:i]]; ok {
					dp[i] = true
					break
				}
			}
		}
	}
	return dp[len(s)]
}
