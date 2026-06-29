package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, m int
	if _, err := fmt.Fscan(in, &n, &m); err != nil {
		return
	}
	nums1 := make([]int, n)
	nums2 := make([]int, m)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums1[i])
	}
	for i := 0; i < m; i++ {
		fmt.Fscan(in, &nums2[i])
	}
	ans := findMedianSortedArraysReference(nums1, nums2)
	fmt.Println(strconv.FormatFloat(ans, 'f', -1, 64))
}

func findMedianSortedArraysReference(nums1 []int, nums2 []int) float64 {
	if len(nums1) > len(nums2) {
		return findMedianSortedArraysReference(nums2, nums1)
	}

	m, n := len(nums1), len(nums2)
	totalLeft := (m + n + 1) / 2
	left, right := 0, m
	for left < right {
		i := left + (right-left)/2
		j := totalLeft - i
		if i < m && j > 0 && nums1[i] < nums2[j-1] {
			left = i + 1
		} else {
			right = i
		}
	}

	i := left
	j := totalLeft - i

	nums1LeftMax := -(1 << 60)
	if i > 0 {
		nums1LeftMax = nums1[i-1]
	}
	nums1RightMin := 1 << 60
	if i < m {
		nums1RightMin = nums1[i]
	}

	nums2LeftMax := -(1 << 60)
	if j > 0 {
		nums2LeftMax = nums2[j-1]
	}
	nums2RightMin := 1 << 60
	if j < n {
		nums2RightMin = nums2[j]
	}

	if (m+n)%2 == 1 {
		return float64(maxReference(nums1LeftMax, nums2LeftMax))
	}
	leftMax := maxReference(nums1LeftMax, nums2LeftMax)
	rightMin := minReference(nums1RightMin, nums2RightMin)
	return float64(leftMax+rightMin) / 2.0
}

func maxReference(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minReference(a, b int) int {
	if a < b {
		return a
	}
	return b
}
