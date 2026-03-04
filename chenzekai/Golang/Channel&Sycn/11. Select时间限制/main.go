package main

import (
    "fmt"
    "time"
)

func main() {
    // 示例1: 将Tick和After放在select内部（错误用法）
    fmt.Println("=== 错误示例：Tick和After在select内部 ===")
    wrongUsage()
    
    // 示例2: 将Tick和After放在select外部（正确用法）
    fmt.Println("\n=== 正确示例：Tick和After在select外部 ===")
    correctUsage()
}

// 错误用法：Tick和After在select内部
func wrongUsage() {
    for {
		select {
    		case <-time.Tick(2 * time.Second):
        		fmt.Printf("2 second over: %d\n", time.Now().Second())
    		case <-time.After(7 * time.Second):
        		fmt.Printf("5 second over, timeout: %d\n", time.Now().Second())
    		}
	}
    // 第一个case在2秒后触发
    // 第二个case永远无法执行
}

// 正确用法：Tick和After在select外部
func correctUsage() {
    tick := time.Tick(1 * time.Second)  // 每秒发送一次
    after := time.After(7 * time.Second) // 7秒后发送一次
    
    fmt.Printf("start second: %d\n", time.Now().Second())
    
    for {
        select {
        case <-tick:
            fmt.Printf("1 second over: %d\n", time.Now().Second())
        case <-after:
            fmt.Printf("7 second over: %d\n", time.Now().Second())
            return
        }
    }
}