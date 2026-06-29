package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, k int
	if _, err := fmt.Fscan(in, &n, &k); err != nil {
		return
	}
	tokens := make([]string, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &tokens[i])
	}
	fmt.Println(kthSmallestReference(buildTreeReference(tokens), k))
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func kthSmallestReference(root *TreeNode, k int) int {
	stack := make([]*TreeNode, 0)
	cur := root
	for cur != nil || len(stack) > 0 {
		for cur != nil {
			stack = append(stack, cur)
			cur = cur.Left
		}
		cur = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		k--
		if k == 0 {
			return cur.Val
		}
		cur = cur.Right
	}
	return 0
}

func buildTreeReference(tokens []string) *TreeNode {
	if len(tokens) == 0 || tokens[0] == "null" {
		return nil
	}
	val, _ := strconv.Atoi(tokens[0])
	root := &TreeNode{Val: val}
	queue := []*TreeNode{root}
	idx := 1
	for len(queue) > 0 && idx < len(tokens) {
		node := queue[0]
		queue = queue[1:]
		if idx < len(tokens) && tokens[idx] != "null" {
			v, _ := strconv.Atoi(tokens[idx])
			node.Left = &TreeNode{Val: v}
			queue = append(queue, node.Left)
		}
		idx++
		if idx < len(tokens) && tokens[idx] != "null" {
			v, _ := strconv.Atoi(tokens[idx])
			node.Right = &TreeNode{Val: v}
			queue = append(queue, node.Right)
		}
		idx++
	}
	return root
}
