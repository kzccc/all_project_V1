package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var rows, cols int
	if _, err := fmt.Fscan(in, &rows, &cols); err != nil {
		return
	}
	grid := make([][]byte, rows)
	for i := 0; i < rows; i++ {
		grid[i] = make([]byte, cols)
		for j := 0; j < cols; j++ {
			var cell string
			fmt.Fscan(in, &cell)
			grid[i][j] = cell[0]
		}
	}
	fmt.Println(numIslandsReference(grid))
}

func numIslandsReference(grid [][]byte) int {
	rows, cols := len(grid), len(grid[0])
	var dfs func(int, int)
	dfs = func(r int, c int) {
		if r < 0 || r >= rows || c < 0 || c >= cols || grid[r][c] != '1' {
			return
		}
		grid[r][c] = '0'
		dfs(r+1, c)
		dfs(r-1, c)
		dfs(r, c+1)
		dfs(r, c-1)
	}
	count := 0
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if grid[i][j] == '1' {
				count++
				dfs(i, j)
			}
		}
	}
	return count
}
