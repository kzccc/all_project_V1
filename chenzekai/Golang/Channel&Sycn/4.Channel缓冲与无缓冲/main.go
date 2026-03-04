package main

import (
    "fmt"
    "time"
)

func main() {
    fmt.Println("=== 演示 Unbuffered Channel ===")
    demoUnbuffered()

    fmt.Println("\n=== 演示 Buffered Channel ===")
    demoBuffered()

    fmt.Println("\n=== 演示 Channel 的 capacity 和 length ===")
    demoCapAndLen()

    fmt.Println("\n=== 演示 close 后不能 send ===")
    demoClose()
}

// demoUnbuffered: 无缓冲通道 —— 同步阻塞
func demoUnbuffered() {
    ch := make(chan int) // 无缓冲

    go func() {
        fmt.Println("sender: 发送数据 42")
        ch <- 42
        fmt.Println("sender: 数据已发送")
    }()

    time.Sleep(100 * time.Millisecond) // 让 sender 先启动

    data := <-ch
    fmt.Println("receiver: 收到数据:", data)
}

// demoBuffered: 有缓冲通道 —— 异步非阻塞
func demoBuffered() {
    ch := make(chan int, 3) // 缓冲区大小为 3

    go func() {
        for i := 1; i <= 5; i++ {
            fmt.Printf("sender: 发送 %d (缓冲区大小: %d/%d)\n", i, len(ch), cap(ch))
            ch <- i
        }
        close(ch)
    }()

    time.Sleep(100 * time.Millisecond)

    for data := range ch {
        fmt.Printf("receiver: 收到 %d\n", data)
    }
}

// demoCapAndLen: 展示 capacity 和 length
func demoCapAndLen() {
    ch := make(chan int, 2)
    fmt.Printf("初始: len=%d, cap=%d\n", len(ch), cap(ch))

    ch <- 1
    fmt.Printf("发送 1 后: len=%d, cap=%d\n", len(ch), cap(ch))

    ch <- 2
    fmt.Printf("发送 2 后: len=%d, cap=%d\n", len(ch), cap(ch))

    data := <-ch

    fmt.Printf("接收 1 个%v后: len=%d, cap=%d\n", data, len(ch), cap(ch))

    close(ch)
}

// demoClose: 关闭后不能 send
func demoClose() {
    ch := make(chan int, 1)
    go func() {
        ch <- 1
        close(ch)
        // ❌ 下面这行会 panic！
        // ch <- 2 // panic: send on closed channel
    }()

    time.Sleep(100 * time.Millisecond)
    data, ok := <-ch
    fmt.Printf("接收数据: %d, ok=%t\n", data, ok)
}