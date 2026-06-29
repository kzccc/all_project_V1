package main

import "sort"

func groupAnagrams(strs []string) [][]string {
	groups := make(map[string][]string, len(strs))
	for _, s := range strs {
		key := sortString(s)
		groups[key] = append(groups[key], s)
	}
	ans := make([][]string, 0, len(groups))
	for _, group := range groups {
		ans = append(ans, group)
	}
	return ans
}

func sortString(s string) string {
	chars := []byte(s)
	sort.Slice(chars, func(i, j int) bool {
		return chars[i] < chars[j]
	})
	return string(chars)
}
