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
	fmt.Println(maxSubArrayReference(nums))
}

func maxSubArrayReference(nums []int) int {
	best := nums[0]
	cur := nums[0]
	for i := 1; i < len(nums); i++ {
		if cur > 0 {
			cur += nums[i]
		} else {
			cur = nums[i]
		}
		if cur > best {
			best = cur
		}
	}
	return best
}
