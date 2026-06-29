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
	var n, target int
	if _, err := fmt.Fscan(in, &n, &target); err != nil {
		return
	}
	candidates := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &candidates[i])
	}
	ans := combinationSumReference(candidates, target)
	fmt.Print(formatIntMatrix(ans))
}

func combinationSumReference(candidates []int, target int) [][]int {
	sort.Ints(candidates)
	path := make([]int, 0)
	ans := make([][]int, 0)
	var dfs func(int, int)
	dfs = func(start, remain int) {
		if remain == 0 {
			ans = append(ans, append([]int(nil), path...))
			return
		}
		for i := start; i < len(candidates); i++ {
			if candidates[i] > remain {
				break
			}
			path = append(path, candidates[i])
			dfs(i, remain-candidates[i])
			path = path[:len(path)-1]
		}
	}
	dfs(0, target)
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
