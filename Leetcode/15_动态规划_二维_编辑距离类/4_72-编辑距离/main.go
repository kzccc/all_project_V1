package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var word1, word2 string
	if _, err := fmt.Fscan(in, &word1, &word2); err != nil {
		return
	}
	fmt.Println(minDistanceReference(word1, word2))
}

func minDistanceReference(word1 string, word2 string) int {
	m, n := len(word1), len(word2)
	dp := make([][]int, m+1)
	for i := 0; i <= m; i++ {
		dp[i] = make([]int, n+1)
	}
	for i := 0; i <= m; i++ {
		dp[i][0] = i
	}
	for j := 0; j <= n; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if word1[i-1] == word2[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = minIntReference(
					dp[i-1][j-1],
					minIntReference(dp[i-1][j], dp[i][j-1]),
				) + 1
			}
		}
	}
	return dp[m][n]
}

func minIntReference(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxIntReference(a, b int) int {
	if a > b {
		return a
	}
	return b
}
