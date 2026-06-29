package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		return
	}
	ans := solveNQueensReference(n)
	fmt.Print(formatBoardSolutions(ans))
}

func solveNQueensReference(n int) [][]string {
	cols := make([]bool, n)
	diag1 := make(map[int]bool)
	diag2 := make(map[int]bool)
	board := make([][]byte, n)
	for i := range board {
		board[i] = []byte(strings.Repeat(".", n))
	}
	ans := make([][]string, 0)
	var dfs func(int)
	dfs = func(row int) {
		if row == n {
			cur := make([]string, n)
			for i := 0; i < n; i++ {
				cur[i] = string(board[i])
			}
			ans = append(ans, cur)
			return
		}
		for col := 0; col < n; col++ {
			if cols[col] || diag1[row-col] || diag2[row+col] {
				continue
			}
			cols[col] = true
			diag1[row-col] = true
			diag2[row+col] = true
			board[row][col] = 'Q'
			dfs(row + 1)
			board[row][col] = '.'
			cols[col] = false
			delete(diag1, row-col)
			delete(diag2, row+col)
		}
	}
	dfs(0)
	return ans
}

func formatBoardSolutions(data [][]string) string {
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, fmt.Sprintf("%d", len(data)))
	for _, sol := range data {
		lines = append(lines, strings.Join(sol, "|"))
	}
	return strings.Join(lines, "\n") + "\n"
}
