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
		return
	}
	fmt.Println(decodeStringReference(s))
}

func decodeStringReference(s string) string {
	countStack := make([]int, 0)
	stringStack := make([]string, 0)
	current := ""
	count := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= '0' && ch <= '9' {
			count = count*10 + int(ch-'0')
			continue
		}
		if ch == '[' {
			countStack = append(countStack, count)
			stringStack = append(stringStack, current)
			count = 0
			current = ""
			continue
		}
		if ch == ']' {
			repeat := countStack[len(countStack)-1]
			countStack = countStack[:len(countStack)-1]

			prev := stringStack[len(stringStack)-1]
			stringStack = stringStack[:len(stringStack)-1]

			current = prev + strings.Repeat(current, repeat)
			continue
		}
		current += string(ch)
	}
	return current
}
