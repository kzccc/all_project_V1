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
	fmt.Println(longestConsecutiveReference(nums))
}

func longestConsecutiveReference(nums []int) int {
	set := make(map[int]bool, len(nums))
	for _, num := range nums {
		set[num] = true
	}
	best := 0
	for num := range set {
		if set[num-1] {
			continue
		}
		length := 1
		cur := num
		for set[cur+1] {
			cur++
			length++
		}
		if length > best {
			best = length
		}
	}
	return best
}
