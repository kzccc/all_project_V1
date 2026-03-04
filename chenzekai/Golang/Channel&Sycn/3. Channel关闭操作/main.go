//?🔥 注意：如果在 close(ch) 后继续 send，程序会 panic。
//?所以必须确保只在 sender 端关闭 channel。
package main

import (
    "fmt"
	"time"
)

func main() {
    ch := make(chan string, 2) // 带缓冲的 channel，容量为 2

    go sender(ch)
    go receiver(ch)

    time.Sleep(2 * time.Second) // 等待 goroutine 执行完毕
}

func sender(ch chan string) {
    fmt.Println("sender: 发送数据")
    ch <- "hello"
    ch <- "world"
    fmt.Println("sender: 关闭 channel")
    close(ch) // 关闭 channel
    // 如果再发送，会 panic！
    // ch <- "panic" // 这行会触发 panic
}

func receiver(ch chan string) {
    for {
        val, ok := <-ch
        if !ok {
            fmt.Println("receiver: channel 已关闭，收到零值:", val)
            break
        }
        fmt.Println("receiver: 收到数据:", val)
    }
}