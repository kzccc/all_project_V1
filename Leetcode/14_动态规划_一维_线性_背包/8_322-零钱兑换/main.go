package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, amount int
	if _, err := fmt.Fscan(in, &n, &amount); err != nil {
		return
	}
	coins := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &coins[i])
	}
	fmt.Println(coinChangeReference(coins, amount))
}

func coinChangeReference(coins []int, amount int) int {
	const inf = int(1e9)
	dp := make([]int, amount+1)
	for i := 1; i <= amount; i++ {
		dp[i] = inf
	}
	for _, coin := range coins {
		for x := coin; x <= amount; x++ {
			dp[x] = minIntReference(dp[x], dp[x-coin]+1)
		}
	}
	if dp[amount] == inf {
		return -1
	}
	return dp[amount]
}

func minIntReference(a, b int) int {
	if a < b {
		return a
	}
	return b
}
