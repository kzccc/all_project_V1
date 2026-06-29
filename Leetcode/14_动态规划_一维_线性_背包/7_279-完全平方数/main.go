package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		return
	}
	fmt.Println(numSquaresReference(n))
}

func numSquaresReference(n int) int {
	dp := make([]int, n+1)
	for i := 1; i <= n; i++ {
		dp[i] = i
	}

	squares := make([]int, 0)
	for j := 1; j*j <= n; j++ {
		squares = append(squares, j*j)
	}

	for _, square := range squares {
		for x := square; x <= n; x++ {
			dp[x] = minIntReference(dp[x], dp[x-square]+1)
		}
	}
	return dp[n]
}

func minIntReference(a, b int) int {
	if a < b {
		return a
	}
	return b
}
