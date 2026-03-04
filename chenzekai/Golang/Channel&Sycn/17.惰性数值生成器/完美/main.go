// 完美的示例：每个生成器都有自己的通道，避免相互影响
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
    // 创建两个独立的生成器
    gen1 := generateNums()
    gen2 := generateNums()
    
    // 每个goroutine使用自己的生成器
    go func() {
        fmt.Println("Goroutine 1:", getNums(gen1))
        fmt.Println("Goroutine 1:", getNums(gen1))
    }()
    
    go func() {
        fmt.Println("Goroutine 2:", getNums(gen2))
        fmt.Println("Goroutine 2:", getNums(gen2))
    }()
    
    time.Sleep(time.Second)
}