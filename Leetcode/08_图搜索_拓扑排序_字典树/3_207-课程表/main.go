package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	var numCourses, m int
	if _, err := fmt.Fscan(in, &numCourses, &m); err != nil {
		return
	}
	prerequisites := make([][]int, m)
	for i := 0; i < m; i++ {
		prerequisites[i] = make([]int, 2)
		fmt.Fscan(in, &prerequisites[i][0], &prerequisites[i][1])
	}
	fmt.Println(canFinishReference(numCourses, prerequisites))
}

func canFinishReference(numCourses int, prerequisites [][]int) bool {
	graph := make([][]int, numCourses)
	indegree := make([]int, numCourses)
	for _, edge := range prerequisites {
		a, b := edge[0], edge[1]
		graph[b] = append(graph[b], a)
		indegree[a]++
	}
	queue := make([]int, 0)
	for i := 0; i < numCourses; i++ {
		if indegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	seen := 0
	for len(queue) > 0 {
		course := queue[0]
		queue = queue[1:]
		seen++
		for _, next := range graph[course] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return seen == numCourses
}
