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
	var n, k int
	if _, err := fmt.Fscan(in, &n, &k); err != nil {
		return
	}
	values := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &values[i])
	}
	fmt.Print(formatListReference(reverseKGroupReference(buildListReference(values), k)))
}

type ListNode struct {
	Val  int
	Next *ListNode
}

func reverseKGroupReference(head *ListNode, k int) *ListNode {
	dummy := &ListNode{Next: head}
	groupPrev := dummy
	for {
		kth := groupPrev
		for i := 0; i < k && kth != nil; i++ {
			kth = kth.Next
		}
		if kth == nil {
			break
		}
		groupNext := kth.Next
		prev, cur := groupNext, groupPrev.Next
		for cur != groupNext {
			next := cur.Next
			cur.Next = prev
			prev = cur
			cur = next
		}
		oldStart := groupPrev.Next
		groupPrev.Next = kth
		groupPrev = oldStart
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
