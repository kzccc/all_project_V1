package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	var q int
	if _, err := fmt.Fscan(in, &q); err != nil {
		return
	}

	stack := ConstructorReference()
	for i := 0; i < q; i++ {
		var op string
		fmt.Fscan(in, &op)
		switch op {
		case "push":
			var x int
			fmt.Fscan(in, &x)
			stack.Push(x)
		case "pop":
			stack.Pop()
		case "top":
			fmt.Fprintln(out, stack.Top())
		case "getMin":
			fmt.Fprintln(out, stack.GetMin())
		}
	}
}

type MinStackReference struct {
	data []int
	mins []int
}

func ConstructorReference() MinStackReference {
	return MinStackReference{}
}

func (s *MinStackReference) Push(val int) {
	s.data = append(s.data, val)
	if len(s.mins) == 0 || val <= s.mins[len(s.mins)-1] {
		s.mins = append(s.mins, val)
	}
}

func (s *MinStackReference) Pop() {
	if len(s.data) == 0 {
		return
	}
	top := s.data[len(s.data)-1]
	s.data = s.data[:len(s.data)-1]
	if top == s.mins[len(s.mins)-1] {
		s.mins = s.mins[:len(s.mins)-1]
	}
}

func (s *MinStackReference) Top() int {
	if len(s.data) == 0 {
		return 0
	}
	return s.data[len(s.data)-1]
}

func (s *MinStackReference) GetMin() int {
	if len(s.mins) == 0 {
		return 0
	}
	return s.mins[len(s.mins)-1]
}
