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
	var target int
	fmt.Fscan(in, &target)
	ans := twoSumReference(nums, target)
	if len(ans) == 2 {
		fmt.Printf("%d %d\n", ans[0], ans[1])
	}
}

func twoSumReference(nums []int, target int) []int {
	indexByValue := make(map[int]int, len(nums))
	for i, num := range nums {
		if j, ok := indexByValue[target-num]; ok {
			return []int{j, i}
		}
		indexByValue[num] = i
	}
	return nil
}
