package main
import (
	"os"
	"bufio"
	"fmt"
)

func solution(n int) int {
	if n<2 {
		return 1
	}
	prev1, prev2 := 2,1
	for i := 3;i <= n; i++ {
		prev1,prev2 = prev1+prev2,prev1
	}
	return prev1
}
func mian(){
	in := bufio.NewReader(os.Stdin)
	var n int
	if _ , err := fmt.Fscan(in,&n); err != nil {
		return 
	}
	fmt.Println(solution(n))
}