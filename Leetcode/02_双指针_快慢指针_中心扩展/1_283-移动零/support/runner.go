package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func main() {
	nums, err := readInput(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	moveZeroes(nums)
	fmt.Println(formatOutput(nums))
}

func readInput(r io.Reader) ([]int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)

	values := make([]int, 0)
	for scanner.Scan() {
		v, err := strconv.Atoi(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q", scanner.Text())
		}
		values = append(values, v)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	n := values[0]
	if n < 0 {
		return nil, fmt.Errorf("array length must be non-negative")
	}
	if len(values) != n+1 {
		return nil, fmt.Errorf("expected %d numbers after length, got %d", n, len(values)-1)
	}

	nums := make([]int, n)
	copy(nums, values[1:])
	return nums, nil
}

func formatOutput(nums []int) string {
	parts := make([]string, len(nums))
	for i, v := range nums {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, " ")
}
