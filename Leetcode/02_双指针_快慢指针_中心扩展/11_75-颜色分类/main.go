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
	sortColorsReference(nums)
	for i, v := range nums {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println()
}

func sortColorsReference(nums []int) {
	zero, i, two := 0, 0, len(nums)-1
	for i <= two {
		if nums[i] == 0 {
			nums[zero], nums[i] = nums[i], nums[zero]
			zero++
			i++
		} else if nums[i] == 2 {
			nums[two], nums[i] = nums[i], nums[two]
			two--
		} else {
			i++
		}
	}
}
