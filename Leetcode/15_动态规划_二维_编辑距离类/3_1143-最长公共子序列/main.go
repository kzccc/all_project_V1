package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var text1, text2 string
	if _, err := fmt.Fscan(in, &text1, &text2); err != nil {
		return
	}
	fmt.Println(longestCommonSubsequenceReference(text1, text2))
}

func longestCommonSubsequenceReference(text1 string, text2 string) int {
	m, n := len(text1), len(text2)
	dp := make([][]int, m+1)
	for i := 0; i <= m; i++ {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if text1[i-1] == text2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = maxIntReference(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	return dp[m][n]
}

func maxIntReference(a, b int) int {
	if a > b {
		return a
	}
	return b
}
