package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		return
	}
	intervals := make([][]int, n)
	for i := 0; i < n; i++ {
		intervals[i] = make([]int, 2)
		fmt.Fscan(in, &intervals[i][0], &intervals[i][1])
	}
	ans := mergeReference(intervals)
	for _, interval := range ans {
		fmt.Println(interval[0], interval[1])
	}
}

func mergeReference(intervals [][]int) [][]int {
	if len(intervals) == 0 {
		return nil
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})
	merged := [][]int{intervals[0]}
	for i := 1; i < len(intervals); i++ {
		last := merged[len(merged)-1]
		if intervals[i][0] <= last[1] {
			if intervals[i][1] > last[1] {
				last[1] = intervals[i][1]
			}
		} else {
			merged = append(merged, intervals[i])
		}
	}
	return merged
}
