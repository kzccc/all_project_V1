package main

import (
    "fmt"
    "sync"
)
//?关于struct封装的方法
// 推荐的做法: Mutex和受保护的数据在一起
type ProtectedData struct {
    mu    sync.Mutex
    value int
}

func (p *ProtectedData) SafeIncrement() {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.value++
}

func (p *ProtectedData) SafeRead() int {
    p.mu.Lock()
    defer p.mu.Unlock()
    return p.value
}

// 使用
func main() {
    data := &ProtectedData{}
    data.SafeIncrement()
    fmt.Println(data.SafeRead())
}