package main

func maxArea(height []int) int {
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
