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
	fmt.Println(maxPathSumReference(buildTreeReference(tokens)))
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func maxPathSumReference(root *TreeNode) int {
	best := -(1 << 60)
	var gain func(*TreeNode) int
	gain = func(node *TreeNode) int {
		if node == nil {
			return 0
		}
		left := maxZeroReference(gain(node.Left))
		right := maxZeroReference(gain(node.Right))
		path := node.Val + left + right
		if path > best {
			best = path
		}
		if left > right {
			return node.Val + left
		}
		return node.Val + right
	}
	gain(root)
	return best
}

func maxZeroReference(x int) int {
	if x > 0 {
		return x
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
