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
	var n, target int
	if _, err := fmt.Fscan(in, &n, &target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	candidates := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &candidates[i])
	}
	fmt.Print(formatIntMatrix(combinationSum(candidates, target)))
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
