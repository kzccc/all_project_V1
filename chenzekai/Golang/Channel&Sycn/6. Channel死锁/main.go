package main

import (
    "fmt"
    "time"
)

func main() {
    
    fmt.Println("=== 演示死锁情况 ===")
    deadlockDemo()

    fmt.Println("\n=== 演示解决死锁方案1：使用goroutine ===")
    solutionWithGoroutine()

    fmt.Println("\n=== 演示解决死锁方案2：使用buffered channel ===")
    solutionWithBufferedChannel()

    time.Sleep(100 * time.Millisecond) // 给goroutine一些时间执行
}

func deadlockDemo() {
    fmt.Println("开始演示死锁情况...")
    fmt.Println("创建一个无缓冲通道，在同一线程中先发送后接收...")
    fmt.Println("注意：为了防止程序崩溃，这里不会实际执行会导致死锁的代码")
    fmt.Println("实际的死锁代码如下：")
    fmt.Println(`
ch := make(chan int) // unbuffered channel
ch <- 42     // 发送会阻塞，因为没人接收
fmt.Println(<-ch) // 接收永远等不到数据`)
    fmt.Println("这会导致：fatal error: all goroutines are asleep - deadlock!")
}

func solutionWithGoroutine() {
    ch := make(chan int)

    go func() {
        ch <- 42 // 在另一个 goroutine 中发送
    }()

    result := <-ch
    fmt.Println("solutionWithGoroutine:", result) // 主 goroutine 接收
}

func solutionWithBufferedChannel() {
    ch := make(chan int, 1) // buffered channel，容量为 1

    ch <- 42     // 不会阻塞，因为缓冲区有空间
    result := <-ch
    fmt.Println("solutionWithBufferedChannel:", result) // 可以正常接收
}