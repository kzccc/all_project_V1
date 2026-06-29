package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
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
	fmt.Print(formatTriplets(threeSumReference(nums)))
}

func threeSumReference(nums []int) [][]int {
	sort.Ints(nums)
	ans := make([][]int, 0)
	for i := 0; i < len(nums); i++ {
		if i > 0 && nums[i] == nums[i-1] {
			continue
		}
		left, right := i+1, len(nums)-1
		for left < right {
			sum := nums[i] + nums[left] + nums[right]
			if sum == 0 {
				ans = append(ans, []int{nums[i], nums[left], nums[right]})
				left++
				right--
				for left < right && nums[left] == nums[left-1] {
					left++
				}
				for left < right && nums[right] == nums[right+1] {
					right--
				}
			} else if sum < 0 {
				left++
			} else {
				right--
			}
		}
	}
	return ans
}

func formatTriplets(data [][]int) string {
	for _, row := range data {
		sort.Ints(row)
	}
	sort.Slice(data, func(i, j int) bool {
		for k := 0; k < len(data[i]) && k < len(data[j]); k++ {
			if data[i][k] != data[j][k] {
				return data[i][k] < data[j][k]
			}
		}
		return len(data[i]) < len(data[j])
	})
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, strconv.Itoa(len(data)))
	for _, row := range data {
		parts := make([]string, 0, len(row)+1)
		parts = append(parts, strconv.Itoa(len(row)))
		for _, v := range row {
			parts = append(parts, strconv.Itoa(v))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n") + "\n"
}
