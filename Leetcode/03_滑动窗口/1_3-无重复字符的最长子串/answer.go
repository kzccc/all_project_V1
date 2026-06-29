package main

func lengthOfLongestSubstring(s string) int {
	lastIndex := make(map[byte]int, len(s))
	left := 0
	best := 0
	for right := 0; right < len(s); right++ {
		if prev, ok := lastIndex[s[right]]; ok && prev >= left {
			left = prev + 1
		}
		length := right - left + 1
		if length > best {
			best = length
		}
		lastIndex[s[right]] = right
	}
	return best
}
