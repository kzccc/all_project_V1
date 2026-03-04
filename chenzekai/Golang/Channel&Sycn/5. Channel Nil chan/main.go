package main

import (
    "fmt"
    "math/rand"
    "time"
)

// 不断向channel c中发送[0,10)的随机数
func send(c chan int) {
    for {
        c <- rand.Intn(10)
    }
}

func add(c chan int) {
    sum := 0
    
    // 1秒后，将向t.C通道发送时间点，使其可读
    t := time.NewTimer(1 * time.Second)
    
    for {
        select {
        case input := <-c:
            // 不断读取c中的随机数据进行加总
            sum = sum + input
        case <-t.C:
            c = nil
            fmt.Println(sum)
        }
    }
}

func main() {
    // 🌟 1. nil channel 演示
    var ch1 chan int // nil channel，未 make
    fmt.Println("ch1 is nil:", ch1 == nil)

    // ❌ 尝试向 nil channel 发送数据 → 程序会永远阻塞！
    // go func() { ch1 <- 1 }() // 不要运行这行！

    // ✅ 但可以安全地检查是否为 nil
    if ch1 == nil {
        fmt.Println("ch1 是 nil channel，不能读写")
    }

	// 🌟 2. 使用nil channel通道实现阻塞 演示
    // 启动add和send函数
    c := make(chan int)
    go add(c)
    go send(c)
    
    // 给3秒时间让前两个goroutine有足够时间运行
    time.Sleep(3 * time.Second)
}