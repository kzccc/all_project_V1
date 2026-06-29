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
	grid := make([][]int, rows)
	for i := 0; i < rows; i++ {
		grid[i] = make([]int, cols)
		for j := 0; j < cols; j++ {
			fmt.Fscan(in, &grid[i][j])
		}
	}
	fmt.Println(orangesRottingReference(grid))
}

type point struct{ r, c int }

func orangesRottingReference(grid [][]int) int {
	rows, cols := len(grid), len(grid[0])
	queue := make([]point, 0)
	fresh := 0
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if grid[i][j] == 2 {
				queue = append(queue, point{i, j})
			} else if grid[i][j] == 1 {
				fresh++
			}
		}
	}
	minutes := 0
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 && fresh > 0 {
		size := len(queue)
		for i := 0; i < size; i++ {
			cur := queue[0]
			queue = queue[1:]
			for _, d := range dirs {
				nr, nc := cur.r+d[0], cur.c+d[1]
				if nr >= 0 && nr < rows && nc >= 0 && nc < cols && grid[nr][nc] == 1 {
					grid[nr][nc] = 2
					fresh--
					queue = append(queue, point{nr, nc})
				}
			}
		}
		minutes++
	}
	if fresh > 0 {
		return -1
	}
	return minutes
}
