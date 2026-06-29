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
	prices := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &prices[i])
	}
	fmt.Println(maxProfitReference(prices))
}

func maxProfitReference(prices []int) int {
	if len(prices) == 0 {
		return 0
	}

	minPrice := prices[0]
	best := 0
	for i := 1; i < len(prices); i++ {
		profit := prices[i] - minPrice
		if profit > best {
			best = profit
		}
		if prices[i] < minPrice {
			minPrice = prices[i]
		}
	}
	return best
}
