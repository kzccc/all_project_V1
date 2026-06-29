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
	matrix := make([][]int, rows)
	for i := 0; i < rows; i++ {
		matrix[i] = make([]int, cols)
		for j := 0; j < cols; j++ {
			fmt.Fscan(in, &matrix[i][j])
		}
	}
	setZeroesReference(matrix)
	for _, row := range matrix {
		for j, v := range row {
			if j > 0 {
				fmt.Print(" ")
			}
			fmt.Print(v)
		}
		fmt.Println()
	}
}

func setZeroesReference(matrix [][]int) {
	rows, cols := len(matrix), len(matrix[0])
	firstColZero := false
	for i := 0; i < rows; i++ {
		if matrix[i][0] == 0 {
			firstColZero = true
		}
		for j := 1; j < cols; j++ {
			if matrix[i][j] == 0 {
				matrix[i][0] = 0
				matrix[0][j] = 0
			}
		}
	}
	for i := rows - 1; i >= 0; i-- {
		for j := cols - 1; j >= 1; j-- {
			if matrix[i][0] == 0 || matrix[0][j] == 0 {
				matrix[i][j] = 0
			}
		}
		if firstColZero {
			matrix[i][0] = 0
		}
	}
}
