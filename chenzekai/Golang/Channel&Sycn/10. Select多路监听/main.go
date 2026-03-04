package main

import (
    "fmt"
    "time"
)

func main() {
    // 创建两个不同类型的channel
    ch1 := make(chan string)
    ch2 := make(chan int)
    
    // 启动goroutine向ch1发送数据
    go func() {
        time.Sleep(1 * time.Second)
        ch1 <- "来自ch1的消息"
    }()
    
    // 启动goroutine向ch2发送数据
    go func() {
        time.Sleep(2 * time.Second)
        ch2 <- 42
    }()
    
    // 启动一个定时器，用于演示timeout
    timeout := time.After(3 * time.Second)
    
    fmt.Println("开始监听channel...")
    
    // 使用select监听多个channel
    for {
        select {
        case msg := <-ch1:
            fmt.Printf("接收到ch1的数据: %s\n", msg)
        case num := <-ch2:
            fmt.Printf("接收到ch2的数据: %d\n", num)
        case <-timeout:
            fmt.Println("超时了，退出select循环")
            return
        default:
            // 非阻塞操作，当没有数据可读时执行
            fmt.Println("暂时没有数据，执行默认操作")
            time.Sleep(100 * time.Millisecond)
        }
    }
}