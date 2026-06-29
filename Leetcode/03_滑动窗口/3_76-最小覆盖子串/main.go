package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var s, t string
	if _, err := fmt.Fscan(in, &s, &t); err != nil {
		return
	}
	fmt.Println(minWindowReference(s, t))
}

func minWindowReference(s string, t string) string {
	if len(t) == 0 || len(s) < len(t) {
		return ""
	}
	need := make(map[byte]int)
	for i := 0; i < len(t); i++ {
		need[t[i]]++
	}
	required := len(need)
	formed := 0
	window := make(map[byte]int)
	bestLen := len(s) + 1
	bestL := 0
	left := 0
	for right := 0; right < len(s); right++ {
		ch := s[right]
		window[ch]++
		if need[ch] > 0 && window[ch] == need[ch] {
			formed++
		}
		for formed == required {
			if right-left+1 < bestLen {
				bestLen = right - left + 1
				bestL = left
			}
			drop := s[left]
			window[drop]--
			if need[drop] > 0 && window[drop] < need[drop] {
				formed--
			}
			left++
		}
	}
	if bestLen == len(s)+1 {
		return ""
	}
	return s[bestL : bestL+bestLen]
}


