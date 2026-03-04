// 不完美的示例：多个goroutine共享同一个生成器通道，导致相互影响
package main

import (
    "fmt"
	"time"
)

func generateNums() chan int {
    nums := make(chan int)
    go func() {
        num := 0
        for {
            nums <- num
            num++
        }
    }()
    return nums
}

func getNums(nums chan int) int {
    return <-nums
}

func main() {
    // 创建一个生成器
    nums := generateNums()
    
    // 多个goroutine同时使用同一个生成器
    go func() {
        fmt.Println("Goroutine 1:", getNums(nums))
        fmt.Println("Goroutine 1:", getNums(nums))
    }()
    
    go func() {
        fmt.Println("Goroutine 2:", getNums(nums))
        fmt.Println("Goroutine 2:", getNums(nums))
    }()
    
    time.Sleep(time.Second)
}