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
	board := make([][]byte, rows)
	for i := 0; i < rows; i++ {
		var s string
		fmt.Fscan(in, &s)
		board[i] = []byte(s)
	}
	var word string
	fmt.Fscan(in, &word)
	if existReference(board, word) {
		fmt.Println("true")
	} else {
		fmt.Println("false")
	}
}

func existReference(board [][]byte, word string) bool {
	cnt := map[byte]int{}
	for _, row := range board {
		for _, c := range row {
			cnt[c]++
		}
	}

	w := []byte(word)
	wordCnt := map[byte]int{}
	for _, c := range w {
		wordCnt[c]++
		if wordCnt[c] > cnt[c] {
			return false
		}
	}
	if cnt[w[len(w)-1]] < cnt[w[0]] {
		for i, j := 0, len(w)-1; i < j; i, j = i+1, j-1 {
			w[i], w[j] = w[j], w[i]
		}
	}

	rows := len(board)
	cols := len(board[0])

	var dfs func(int, int, int) bool
	dfs = func(r, c, idx int) bool {
		if board[r][c] != w[idx] {
			return false
		}
		if idx == len(w)-1 {
			return true
		}
		ch := board[r][c]
		board[r][c] = '#'
		dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

		for _, d := range dirs {
			nr, nc := r+d[0], c+d[1]
			if nr >= 0 && nr < rows && nc >= 0 && nc < cols && board[nr][nc] != '#' {
				if dfs(nr, nc, idx+1) {
					board[r][c] = ch
					return true
				}
			}
		}
		board[r][c] = ch
		return false
	}
	
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if dfs(i, j, 0) {
				return true
			}
		}
	}
	return false
}
