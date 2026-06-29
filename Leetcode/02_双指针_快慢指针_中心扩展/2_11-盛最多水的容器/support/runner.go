package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	height := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &height[i])
	}
	fmt.Println(maxArea(height))
}
