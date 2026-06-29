package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"os"
	"sort"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, k int
	if _, err := fmt.Fscan(in, &n, &k); err != nil {
		return
	}
	nums := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums[i])
	}
	ans := topKFrequentReference(nums, k)
	sort.Ints(ans)
	for i, v := range ans {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println()
}

type freqPair struct {
	num  int
	freq int
}

type freqMinHeap []freqPair

func (h freqMinHeap) Len() int      { return len(h) }
func (h freqMinHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h freqMinHeap) Less(i, j int) bool {
	return h[i].freq < h[j].freq
}
func (h *freqMinHeap) Push(x any) { *h = append(*h, x.(freqPair)) }
func (h *freqMinHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

func topKFrequentReference(nums []int, k int) []int {
	count := make(map[int]int)
	for _, x := range nums {
		count[x]++
	}
	h := &freqMinHeap{}
	for num, freq := range count {
		if h.Len() < k {
			heap.Push(h, freqPair{num, freq})
		} else if freq > (*h)[0].freq {
			(*h)[0] = freqPair{num, freq}
			heap.Fix(h, 0)
		}
	}
	ans := make([]int, h.Len())
	for i := range ans {
		ans[i] = heap.Pop(h).(freqPair).num
	}
	return ans
}
