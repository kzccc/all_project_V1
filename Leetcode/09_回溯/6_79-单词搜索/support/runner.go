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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	board := make([][]byte, rows)
	for i := 0; i < rows; i++ {
		var s string
		fmt.Fscan(in, &s)
		board[i] = []byte(s)
	}
	var word string
	fmt.Fscan(in, &word)
	if exist(board, word) {
		fmt.Println("true")
	} else {
		fmt.Println("false")
	}
}
