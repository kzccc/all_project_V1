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
	preorder := make([]int, n)
	inorder := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &preorder[i])
	}
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &inorder[i])
	}
	fmt.Print(formatTreeReference(buildTreeReference(preorder, inorder)))
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func buildTreeReference(preorder []int, inorder []int) *TreeNode {
	index := make(map[int]int, len(inorder))
	for i, v := range inorder {
		index[v] = i
	}
	var dfs func(int, int, int, int) *TreeNode
	dfs = func(pl int, pr int, il int, ir int) *TreeNode {
		if pl > pr {
			return nil
		}
		rootVal := preorder[pl]
		k := index[rootVal]
		leftSize := k - il
		return &TreeNode{
			Val:   rootVal,
			Left:  dfs(pl+1, pl+leftSize, il, k-1),
			Right: dfs(pl+leftSize+1, pr, k+1, ir),
		}
	}
	return dfs(0, len(preorder)-1, 0, len(inorder)-1)
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
