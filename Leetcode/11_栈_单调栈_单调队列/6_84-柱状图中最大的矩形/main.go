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
	heights := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &heights[i])
	}
	fmt.Println(largestRectangleAreaReference(heights))
}

func largestRectangleAreaReference(heights []int) int {
	stack := make([]int, 0, len(heights)+1)
	maxArea := 0

	for i := 0; i <= len(heights); i++ {
		curHeight := 0
		if i < len(heights) {
			curHeight = heights[i]
		}

		for len(stack) > 0 && heights[stack[len(stack)-1]] > curHeight {
			h := heights[stack[len(stack)-1]]
			stack = stack[:len(stack)-1]

			left := -1
			if len(stack) > 0 {
				left = stack[len(stack)-1]
			}
			width := i - left - 1
			area := h * width
			if area > maxArea {
				maxArea = area
			}
		}

		stack = append(stack, i)
	}
	return maxArea
}
