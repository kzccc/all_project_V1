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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	nums := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums[i])
	}
	var target int
	fmt.Fscan(in, &target)
	ans := twoSum(nums, target)
	if len(ans) != 2 {
		fmt.Fprintln(os.Stderr, "twoSum should return exactly two indices")
		os.Exit(1)
	}
	fmt.Printf("%d %d\n", ans[0], ans[1])
}
