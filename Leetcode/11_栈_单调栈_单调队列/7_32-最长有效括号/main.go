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
	fmt.Println(longestValidParenthesesReference(s))
}

func longestValidParenthesesReference(s string) int {
	stack := []int{-1}
	best := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '(' {
			stack = append(stack, i)
			continue
		}

		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			stack = append(stack, i)
			continue
		}

		length := i - stack[len(stack)-1]
		if length > best {
			best = length
		}
	}
	return best
}
