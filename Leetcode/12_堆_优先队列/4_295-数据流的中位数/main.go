package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	var q int
	if _, err := fmt.Fscan(in, &q); err != nil {
		return
	}
	mf := ConstructorReference()
	for i := 0; i < q; i++ {
		var op string
		fmt.Fscan(in, &op)
		if op == "addNum" {
			var x int
			fmt.Fscan(in, &x)
			mf.AddNum(x)
		} else if op == "findMedian" {
			fmt.Fprintln(out, strconv.FormatFloat(mf.FindMedian(), 'f', -1, 64))
		}
	}
}

type maxHeap []int
type minHeap []int

func (h maxHeap) Len() int           { return len(h) }
func (h maxHeap) Less(i, j int) bool { return h[i] > h[j] }
func (h maxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *maxHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *minHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

type MedianFinderReference struct {
	left  *maxHeap
	right *minHeap
}

func ConstructorReference() MedianFinderReference {
	left := &maxHeap{}
	right := &minHeap{}
	return MedianFinderReference{left: left, right: right}
}

func (m *MedianFinderReference) AddNum(num int) {
	if m.left.Len() == 0 || num <= (*(m.left))[0] {
		heap.Push(m.left, num)
	} else {
		heap.Push(m.right, num)
	}
	if m.left.Len() > m.right.Len()+1 {
		heap.Push(m.right, heap.Pop(m.left))
	}
	if m.right.Len() > m.left.Len() {
		heap.Push(m.left, heap.Pop(m.right))
	}
}

func (m *MedianFinderReference) FindMedian() float64 {
	if m.left.Len() > m.right.Len() {
		return float64((*(m.left))[0])
	}
	return float64((*(m.left))[0]+(*(m.right))[0]) / 2.0
}
