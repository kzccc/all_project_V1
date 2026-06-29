package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var rows, cols, target int
	if _, err := fmt.Fscan(in, &rows, &cols, &target); err != nil {
		return
	}
	matrix := make([][]int, rows)
	for i := 0; i < rows; i++ {
		matrix[i] = make([]int, cols)
		for j := 0; j < cols; j++ {
			fmt.Fscan(in, &matrix[i][j])
		}
	}
	fmt.Println(searchMatrix(matrix, target))
}

func searchMatrix(matrix [][]int, target int) bool {
	row := len(matrix)
	col := len(matrix[0])

	left := 0
	right := row * col

	for left < right {
		mid := left + (right-left)/2
		if matrix[mid/col][mid%col] == target {
			return true
		}
		if matrix[mid/col][mid%col] > target {
			right = mid
		} else {
			left = mid + 1
		}
	}
	return false
}
