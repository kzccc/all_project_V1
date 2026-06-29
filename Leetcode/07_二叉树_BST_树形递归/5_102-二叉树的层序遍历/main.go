package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n int
	if _, err := fmt.Fscan(in, &n); err != nil {
		return
	}
	tokens := make([]string, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &tokens[i])
	}
	ans := levelOrderReference(buildTreeReference(tokens))
	for _, level := range ans {
		for i, v := range level {
			if i > 0 {
				fmt.Print(" ")
			}
			fmt.Print(v)
		}
		fmt.Println()
	}
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func levelOrderReference(root *TreeNode) [][]int {
	if root == nil {
		return nil
	}
	queue := []*TreeNode{root}
	ans := make([][]int, 0)
	for len(queue) > 0 {
		size := len(queue)
		level := make([]int, 0, size)
		for i := 0; i < size; i++ {
			node := queue[0]
			queue = queue[1:]
			level = append(level, node.Val)
			if node.Left != nil {
				queue = append(queue, node.Left)
			}
			if node.Right != nil {
				queue = append(queue, node.Right)
			}
		}
		ans = append(ans, level)
	}
	return ans
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
