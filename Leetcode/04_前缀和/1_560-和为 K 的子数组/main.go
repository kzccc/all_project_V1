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
	fmt.Println(subarraySumReference(nums, k))
}

func subarraySumReference(nums []int, k int) int {
	count := map[int]int{0: 1}
	sum := 0
	ans := 0
	for _, x := range nums {
		sum += x
		ans += count[sum-k]
		count[sum]++
	}
	return ans
}
