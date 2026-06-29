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
	randomIdx := make([]int, n)
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &values[i])
	}
	for i := 0; i < n; i++ {
		fmt.Fscan(in, &randomIdx[i])
	}
	head := buildRandomListReference(values, randomIdx)
	copied := copyRandomListReference(head)
	fmt.Print(formatRandomListReference(copied))
}

type Node struct {
	Val    int
	Next   *Node
	Random *Node
}

func copyRandomListReference(head *Node) *Node {
	if head == nil {
		return nil
	}
	nodeMap := make(map[*Node]*Node)
	for cur := head; cur != nil; cur = cur.Next {
		nodeMap[cur] = &Node{Val: cur.Val}
	}
	for cur := head; cur != nil; cur = cur.Next {
		nodeMap[cur].Next = nodeMap[cur.Next]
		nodeMap[cur].Random = nodeMap[cur.Random]
	}
	return nodeMap[head]
}

func buildRandomListReference(values []int, randomIdx []int) *Node {
	if len(values) == 0 {
		return nil
	}
	nodes := make([]*Node, len(values))
	for i, v := range values {
		nodes[i] = &Node{Val: v}
		if i > 0 {
			nodes[i-1].Next = nodes[i]
		}
	}
	for i, idx := range randomIdx {
		if idx >= 0 {
			nodes[i].Random = nodes[idx]
		}
	}
	return nodes[0]
}

func formatRandomListReference(head *Node) string {
	if head == nil {
		return "0\n"
	}
	nodes := make([]*Node, 0)
	indexMap := make(map[*Node]int)
	for cur := head; cur != nil; cur = cur.Next {
		indexMap[cur] = len(nodes)
		nodes = append(nodes, cur)
	}
	values := make([]string, len(nodes))
	randoms := make([]string, len(nodes))
	for i, node := range nodes {
		values[i] = strconv.Itoa(node.Val)
		if node.Random == nil {
			randoms[i] = "-1"
		} else {
			randoms[i] = strconv.Itoa(indexMap[node.Random])
		}
	}
	return fmt.Sprintf("%d\n%s\n%s\n", len(nodes), strings.Join(values, " "), strings.Join(randoms, " "))
}
