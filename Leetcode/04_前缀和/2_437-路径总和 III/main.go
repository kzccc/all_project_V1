package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var n, target int
	if _, err := fmt.Fscan(in, &n, &target); err != nil {
		return
	}
	tokens := make([]string, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &tokens[i])
	}
	fmt.Println(pathSumReference(buildTreeReference(tokens), target))
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func pathSumReference(root *TreeNode, targetSum int) int {
	count := map[int64]int{0: 1}
	var dfs func(*TreeNode, int64) int
	dfs = func(node *TreeNode, sum int64) int {
		if node == nil {
			return 0
		}
		sum += int64(node.Val)
		ans := count[sum-int64(targetSum)]
		count[sum]++
		ans += dfs(node.Left, sum)
		ans += dfs(node.Right, sum)
		count[sum]--
		return ans
	}
	return dfs(root, 0)
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
