package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var s, p string
	if _, err := fmt.Fscan(in, &s, &p); err != nil {
		return
	}
	ans := findAnagramsReference(s, p)
	for i, v := range ans {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println()
}

func findAnagramsReference(s string, p string) []int {
	if len(p) > len(s) {
		return nil
	}
	need := [26]int{}
	window := [26]int{}
	for i := 0; i < len(p); i++ {
		need[p[i]-'a']++
		window[s[i]-'a']++
	}
	ans := make([]int, 0)
	if need == window {
		ans = append(ans, 0)
	}
	for i := len(p); i < len(s); i++ {
		window[s[i]-'a']++
		window[s[i-len(p)]-'a']--
		if need == window {
			ans = append(ans, i-len(p)+1)
		}
	}
	return ans
}
