package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, k int
	if _, err := fmt.Fscan(in, &n, &k); err != nil {
		return
	}
	nums := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums[i])
	}
	rotateReference(nums, k)
	for i, v := range nums {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println()
}

func rotateReference(nums []int, k int) {
	if len(nums) == 0 {
		return
	}
	k %= len(nums)
	reverseReference(nums, 0, len(nums)-1)
	reverseReference(nums, 0, k-1)
	reverseReference(nums, k, len(nums)-1)
}

func reverseReference(nums []int, left int, right int) {
	for left < right {
		nums[left], nums[right] = nums[right], nums[left]
		left++
		right--
	}
}
