package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var s string
	if _, err := fmt.Fscan(in, &s); err != nil {
		return
	}
	fmt.Println(longestPalindromeReference(s))
}

func longestPalindromeReference(s string) string {
	if len(s) < 2 {
		return s
	}
	bestL, bestR := 0, 0
	for i := 0; i < len(s); i++ {
		l1, r1 := expandReference(s, i, i)
		if r1-l1 > bestR-bestL {
			bestL, bestR = l1, r1
		}
		l2, r2 := expandReference(s, i, i+1)
		if r2-l2 > bestR-bestL {
			bestL, bestR = l2, r2
		}
	}
	return s[bestL : bestR+1]
}

func expandReference(s string, left int, right int) (int, int) {
	for left >= 0 && right < len(s) && s[left] == s[right] {
		left--
		right++
	}
	return left + 1, right - 1
}
