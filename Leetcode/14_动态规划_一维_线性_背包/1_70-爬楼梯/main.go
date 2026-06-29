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
	fmt.Println(climbStairsReference(n))
}

func climbStairsReference(n int) int {
	if n <= 2 {
		return n
	}

	prev2, prev1 := 1, 2
	for i := 3; i <= n; i++ {
		prev2, prev1 = prev1, prev1+prev2
	}
	return prev1
}
