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
	nums := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums[i])
	}
	fmt.Println(maxProductReference(nums))
}

func maxProductReference(nums []int) int {
	maxEnd, minEnd := nums[0], nums[0]
	best := nums[0]
	for i := 1; i < len(nums); i++ {
		x := nums[i]
		if x < 0 {
			maxEnd, minEnd = minEnd, maxEnd
		}
		maxEnd = maxIntReference(x, maxEnd*x)
		minEnd = minIntReference(x, minEnd*x)
		best = maxIntReference(best, maxEnd)
	}
	return best
}

func maxIntReference(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minIntReference(a, b int) int {
	if a < b {
		return a
	}
	return b
}
