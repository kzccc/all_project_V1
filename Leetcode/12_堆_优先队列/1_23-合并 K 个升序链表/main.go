package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var k int
	if _, err := fmt.Fscan(in, &k); err != nil {
		return
	}
	lists := make([]*ListNode, k)
	for i := 0; i < k; i++ {
		var n int
		fmt.Fscan(in, &n)
		values := make([]int, n)
		for j := 0; j < n; j++ {
			fmt.Fscan(in, &values[j])
		}
		lists[i] = buildListReference(values)
	}
	fmt.Print(formatListReference(mergeKListsReference(lists)))
}

type ListNode struct {
	Val  int
	Next *ListNode
}

type nodeHeap []*ListNode

func (h nodeHeap) Len() int           { return len(h) }
func (h nodeHeap) Less(i, j int) bool { return h[i].Val < h[j].Val }
func (h nodeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nodeHeap) Push(x any) {
	*h = append(*h, x.(*ListNode))
}

func (h *nodeHeap) Pop() any {
	old := *h
	node := old[len(old)-1]
	*h = old[:len(old)-1]
	return node
}

func mergeKListsReference(lists []*ListNode) *ListNode {
	h := &nodeHeap{}
	for _, node := range lists {
		if node != nil {
			heap.Push(h, node)
		}
	}
	dummy := &ListNode{}
	tail := dummy
	for h.Len() > 0 {
		node := heap.Pop(h).(*ListNode)
		tail.Next = node
		tail = tail.Next
		if node.Next != nil {
			heap.Push(h, node.Next)
		}
	}
	return dummy.Next
}

func buildListReference(values []int) *ListNode {
	dummy := &ListNode{}
	cur := dummy
	for _, v := range values {
		cur.Next = &ListNode{Val: v}
		cur = cur.Next
	}
	return dummy.Next
}

func formatListReference(head *ListNode) string {
	values := make([]string, 0)
	for cur := head; cur != nil; cur = cur.Next {
		values = append(values, strconv.Itoa(cur.Val))
	}
	if len(values) == 0 {
		return "0\n"
	}
	return fmt.Sprintf("%d %s\n", len(values), strings.Join(values, " "))
}
