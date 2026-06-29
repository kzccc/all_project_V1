package main

import (
	"bufio"
	"fmt"
	"os"
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
	ans := subsetsReference(nums)
	fmt.Print(formatIntMatrix(ans))
}

func subsetsReference(nums []int) [][]int {
	ans := make([][]int, 0)
	path := make([]int, 0)
	var dfs func(int)
	dfs = func(start int) {
		ans = append(ans, append([]int(nil), path...))
		for i := start; i < len(nums); i++ {
			path = append(path, nums[i])
			dfs(i + 1)
			path = path[:len(path)-1]
		}
	}
	dfs(0)
	return ans
}

func formatIntMatrix(data [][]int) string {
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
