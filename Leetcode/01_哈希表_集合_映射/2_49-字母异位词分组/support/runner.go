package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	strs := make([]string, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &strs[i])
	}
	fmt.Print(formatStringGroups(groupAnagrams(strs)))
}

func formatStringGroups(groups [][]string) string {
	for _, group := range groups {
		sort.Strings(group)
	}
	sort.Slice(groups, func(i, j int) bool {
		left := strings.Join(groups[i], "\x00")
		right := strings.Join(groups[j], "\x00")
		return left < right
	})
	lines := make([]string, 0, len(groups)+1)
	lines = append(lines, fmt.Sprintf("%d", len(groups)))
	for _, group := range groups {
		line := fmt.Sprintf("%d", len(group))
		if len(group) > 0 {
			line += " " + strings.Join(group, " ")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}
