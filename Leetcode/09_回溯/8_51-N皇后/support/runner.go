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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(formatBoardSolutions(solveNQueens(n)))
}

func formatBoardSolutions(data [][]string) string {
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, fmt.Sprintf("%d", len(data)))
	for _, sol := range data {
		lines = append(lines, strings.Join(sol, "|"))
	}
	return strings.Join(lines, "\n") + "\n"
}
