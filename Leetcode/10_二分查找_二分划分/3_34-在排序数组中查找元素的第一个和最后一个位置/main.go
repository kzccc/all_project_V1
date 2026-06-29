package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, target int
	if _, err := fmt.Fscan(in, &n, &target); err != nil {
		return
	}
	nums := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums[i])
	}
	ans := searchRangeReference(nums, target)
	fmt.Println(ans[0], ans[1])
}

func searchRangeReference(nums []int, target int) []int {
	left := lowerBoundReference(nums, target)
	if left == len(nums) || nums[left] != target {
		return []int{-1, -1}
	}
	right := lowerBoundReference(nums, target+1) - 1
	return []int{left, right}
}

func lowerBoundReference(nums []int, target int) int {
	left, right := 0, len(nums)
	for left < right {
		mid := left + (right-left)/2
		if nums[mid] < target {
			left = mid + 1
		} else {
			right = mid
		}
	}
	return left
}
