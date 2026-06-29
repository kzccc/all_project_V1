package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		return
	}
	values := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &values[i])
	}
	fmt.Print(formatListReference(swapPairsReference(buildListReference(values))))
}

type ListNode struct {
	Val  int
	Next *ListNode
}

func swapPairsReference(head *ListNode) *ListNode {
	dummy := &ListNode{Next: head}
	prev := dummy
	for prev.Next != nil && prev.Next.Next != nil {
		a := prev.Next
		b := a.Next                     
		prev.Next = b
		a.Next = b.Next
		b.Next = a
		prev = a
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
