package main

import (
    "fmt"
    "sync"
)

func main() {
    // 创建一个无缓冲通道
    count := make(chan int)
    
    // 创建WaitGroup用于等待goroutines完成
    var wg sync.WaitGroup
    
    // 增加两个待等待的goroutines
    wg.Add(2)
    fmt.Println("Start Goroutines")
    
    // 激活两个goroutine
    go printCounts("Goroutine-1", count, &wg)
    go printCounts("Goroutine-2", count, &wg)
    
    fmt.Println("Communication of channel begins")
    
    // 向channel中发送初始数据
    count <- 1
    
    fmt.Println("Waiting To Finish")
    wg.Wait()
    fmt.Println("\nTerminating the Program")
}

// 使用for range迭代channel的版本
//?前面都是在for无限循环中读取channel中的数据，但也可以使用range来迭代channel,它会返回每次迭代过程中所读取的数据，直到channel被关闭。必须注意，只要channel未关闭，range迭代channel就会一直被阻塞。
func printCounts(label string, count chan int, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for val := range count {
        fmt.Printf("Count: %d received from %s \n", val, label)
        
        if val == 10 {
            fmt.Printf("Channel Closed from %s \n", label)
            close(count)
            return
        }
        
        val++
        count <- val
    }
}