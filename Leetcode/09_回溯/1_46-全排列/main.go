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
	ans := permuteReference(nums)
	fmt.Print(formatIntMatrix(ans))
}

func permuteReference(nums []int) [][]int {
	// 标记每个位置的数字是否已被选入当前排列
	used := make([]bool, len(nums))
	// 当前正在构建的排列路径
	path := make([]int, 0, len(nums))
	ans := make([][]int, 0)

	var dfs func()
	dfs = func() {
		// 终止条件：path 长度等于 nums，说明一个完整排列已构造好
		if len(path) == len(nums) {
			// 必须拷贝 path，path 在回溯过程中会被修改
			cur := append([]int(nil), path...)
			ans = append(ans, cur)
			return
		}
		// 遍历所有候选数字，尝试填入当前位置
		for i := 0; i < len(nums); i++ {
			// 已经使用过的数字直接跳过
			if used[i] {
				continue
			}
			// 做选择
			used[i] = true
			path = append(path, nums[i])
			// 递归进入下一层，继续填下一个位置
			dfs()
			// 撤销选择：还原 path 和 used，尝试下一个候选
			path = path[:len(path)-1]
			used[i] = false
		}
	}
	dfs()
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
