package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var lenA, lenB, lenCommon int
	if _, err := fmt.Fscan(in, &lenA, &lenB, &lenCommon); err != nil {
		return
	}
	aVals := make([]int, lenA)
	bVals := make([]int, lenB)
	commonVals := make([]int, lenCommon)
	for i := 0; i < lenA; i++ {
		fmt.Fscan(in, &aVals[i])
	}
	for i := 0; i < lenB; i++ {
		fmt.Fscan(in, &bVals[i])
	}
	for i := 0; i < lenCommon; i++ {
		fmt.Fscan(in, &commonVals[i])
	}
	headA, headB := buildIntersectingListsReference(aVals, bVals, commonVals)
	node := getIntersectionNodeReference(headA, headB)
	if node == nil {
		fmt.Println(-1)
	} else {
		fmt.Println(node.Val)
	}
}

type ListNode struct {
	Val  int
	Next *ListNode
}

func getIntersectionNodeReference(headA *ListNode, headB *ListNode) *ListNode {
	a, b := headA, headB
	for a != b {
		if a == nil {
			a = headB
		} else {
			a = a.Next
		}
		if b == nil {
			b = headA
		} else {
			b = b.Next
		}
	}
	return a
}

func buildIntersectingListsReference(aVals []int, bVals []int, commonVals []int) (*ListNode, *ListNode) {
	commonHead := buildListReference(commonVals)
	headA := appendTailReference(buildListReference(aVals), commonHead)
	headB := appendTailReference(buildListReference(bVals), commonHead)
	return headA, headB
}

func appendTailReference(head *ListNode, tail *ListNode) *ListNode {
	if head == nil {
		return tail
	}
	cur := head
	for cur.Next != nil {
		cur = cur.Next
	}
	cur.Next = tail
	return head
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
