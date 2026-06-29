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
	height := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &height[i])
	}
	fmt.Println(maxAreaReference(height))
}

func maxAreaReference(height []int) int {
	left, right := 0, len(height)-1
	best := 0
	for left < right {
		width := right - left
		h := height[left]
		if height[right] < h {
			h = height[right]
		}
		area := width * h
		if area > best {
			best = area
		}
		if height[left] < height[right] {
			left++
		} else {
			right--
		}
	}
	return best
}
