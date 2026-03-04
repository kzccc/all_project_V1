package main

import "fmt"
import "time"

// 定义接收-only的channel类型
func receiver(in <-chan int) {
    for val := range in {
        fmt.Printf("Received: %d\n", val)
    }
}

// 定义发送-only的channel类型
func sender(out chan<- int) {
    for i := 0; i < 5; i++ {
        out <- i
        fmt.Printf("Sent: %d\n", i)
    }
    close(out)
}

func main() {
    // 创建一个双向channel
    ch := make(chan int, 3)
    
    // 启动接收者goroutine
    go receiver(ch)
    
    // 启动发送者goroutine
    go sender(ch)
    
    // 等待一段时间让程序完成
    fmt.Println("Waiting for completion...")
    time.Sleep(1 * time.Second)
}