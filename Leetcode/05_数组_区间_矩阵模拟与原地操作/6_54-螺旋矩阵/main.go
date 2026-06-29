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
	ans := spiralOrderReference(matrix)
	for i, v := range ans {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println()
}

func spiralOrderReference(matrix [][]int) []int {
	if len(matrix) == 0 {
		return nil
	}
	top, bottom := 0, len(matrix)-1
	left, right := 0, len(matrix[0])-1
	ans := make([]int, 0, len(matrix)*len(matrix[0]))
	for top <= bottom && left <= right {
		for j := left; j <= right; j++ {
			ans = append(ans, matrix[top][j])
		}
		top++
		for i := top; i <= bottom; i++ {
			ans = append(ans, matrix[i][right])
		}
		right--
		if top <= bottom {
			for j := right; j >= left; j-- {
				ans = append(ans, matrix[bottom][j])
			}
			bottom--
		}
		if left <= right {
			for i := bottom; i >= top; i-- {
				ans = append(ans, matrix[i][left])
			}
			left++
		}
	}
	return ans
}
