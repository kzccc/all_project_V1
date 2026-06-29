package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var s string
	if _, err := fmt.Fscan(in, &s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(formatStringMatrix(partition(s)))
}

func formatStringMatrix(data [][]string) string {
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, fmt.Sprintf("%d", len(data)))
	for _, row := range data {
		lines = append(lines, fmt.Sprintf("%d %s", len(row), strings.Join(row, " ")))
	}
	return strings.Join(lines, "\n") + "\n"
}
