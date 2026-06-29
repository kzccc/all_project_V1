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
	head := buildList(values)
	fmt.Print(formatList(reverseListReference(head)))
}

func reverseListReference(head *ListNode) *ListNode {
	var prev *ListNode
	cur := head                                                          
	for cur != nil {
		next := cur.Next
		cur.Next = prev                           
		prev = cur
		cur = next
	}
	return prev
}

func buildList(values []int) *ListNode {
	dummy := &ListNode{}
	tail := dummy
	for _, v := range values {
		tail.Next = &ListNode{Val: v}
		tail = tail.Next
	}
	return dummy.Next
}

func formatList(head *ListNode) string {
	values := make([]string, 0)
	length := 0
	for cur := head; cur != nil; cur = cur.Next {
		values = append(values, strconv.Itoa(cur.Val))
		length++
	}
	if length == 0 {
		return "0\n"
	}
	return fmt.Sprintf("%d %s\n", length, strings.Join(values, " "))
}
