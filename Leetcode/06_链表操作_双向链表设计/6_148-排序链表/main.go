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
	fmt.Print(formatListReference(sortListReference(buildListReference(values))))
}

type ListNode struct {
	Val  int
	Next *ListNode
}

func sortListReference(head *ListNode) *ListNode {
	if head == nil || head.Next == nil {
		return head
	}
	slow, fast := head, head.Next
	for fast != nil && fast.Next != nil {
		slow = slow.Next
		fast = fast.Next.Next
	}
	right := slow.Next
	slow.Next = nil
	left := sortListReference(head)
	right = sortListReference(right)
	return mergeReference(left, right)
}

func mergeReference(a *ListNode, b *ListNode) *ListNode {
	dummy := &ListNode{}
	tail := dummy
	for a != nil && b != nil {
		if a.Val <= b.Val {
			tail.Next = a
			a = a.Next
		} else {
			tail.Next = b
			b = b.Next
		}
		tail = tail.Next
	}
	if a != nil {
		tail.Next = a
	} else {
		tail.Next = b
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
