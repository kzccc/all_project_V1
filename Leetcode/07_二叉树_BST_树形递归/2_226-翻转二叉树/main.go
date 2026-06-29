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
	tokens := make([]string, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &tokens[i])
	}
	root := buildTreeReference(tokens)
	invertTreeReference(root)
	fmt.Print(formatTreeReference(root))
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func invertTreeReference(root *TreeNode) *TreeNode {
    if root == nil {
        return nil
    }
    invert(root)
    return root
}
func invert(root *TreeNode) {
    if root == nil {
        return
    }
    root.Left, root.Right = root.Right, root.Left
    invert(root.Left)
    invert(root.Right)
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

func formatTreeReference(root *TreeNode) string {
	if root == nil {
		return "0\n"
	}
	queue := []*TreeNode{root}
	values := make([]string, 0)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node == nil {
			values = append(values, "null")
			continue
		}
		values = append(values, strconv.Itoa(node.Val))
		queue = append(queue, node.Left, node.Right)
	}
	for len(values) > 0 && values[len(values)-1] == "null" {
		values = values[:len(values)-1]
	}
	return fmt.Sprintf("%d %s\n", len(values), strings.Join(values, " "))
}
