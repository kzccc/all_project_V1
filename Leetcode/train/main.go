package main

import (
	"bufio"
	"fmt"
	"os"
)

func solution(nums []int) int {
	prev2, prev1 := 0, 0
	for _, x := range nums {
		cur := prev1
		if prev2+x > cur {
			cur = prev2 + x
		}
		prev2, prev1 = prev1, cur
	}
	return prev1
}

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		fmt.Println("输入长度错误:", err)
		return
	}

	nums := make([]int, n)
	for i := 0; i < n; i++ {
		if _, err := fmt.Fscan(in, &nums[i]); err != nil {
			fmt.Printf("第 %d 个数字输入错误: %v\n", i+1, err)
			return
		}
	}

	fmt.Println(solution(nums))
}