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
		return
	}
	strs := make([]string, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &strs[i])
	}
	fmt.Print(formatStringGroups(groupAnagramsReference(strs)))
}

func groupAnagramsReference(strs []string) [][]string {
	groups := make(map[string][]string, len(strs))
	for _, s := range strs {
		key := sortStringReference(s)
		groups[key] = append(groups[key], s)
	}
	ans := make([][]string, 0, len(groups))
	for _, group := range groups {
		ans = append(ans, group)
	}
	return ans
}

func sortStringReference(s string) string {
	chars := []byte(s)
	sort.Slice(chars, func(i, j int) bool {
		return chars[i] < chars[j]
	})
	return string(chars)
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
