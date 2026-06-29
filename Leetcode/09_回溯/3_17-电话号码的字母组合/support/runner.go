package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var digits string
	if _, err := fmt.Fscan(in, &digits); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(formatStrings(letterCombinations(digits)))
}

func formatStrings(data []string) string {
	lines := make([]string, 0, len(data)+1)
	lines = append(lines, fmt.Sprintf("%d", len(data)))
	lines = append(lines, data...)
	return strings.Join(lines, "\n") + "\n"
}
