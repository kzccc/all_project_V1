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
	nums := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &nums[i])
	}
	fmt.Print(formatTreeReference(sortedArrayToBSTReference(nums)))
}

type TreeNode struct {
	Val   int
	Left  *TreeNode
	Right *TreeNode
}

func sortedArrayToBSTReference(nums []int) *TreeNode {
	var build func(int, int) *TreeNode
	build = func(left int, right int) *TreeNode {
		if left > right {
			return nil
		}
		mid := left + (right-left)/2
		return &TreeNode{
			Val:   nums[mid],
			Left:  build(left, mid-1),
			Right: build(mid+1, right),
		}
	}
	return build(0, len(nums)-1)
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
