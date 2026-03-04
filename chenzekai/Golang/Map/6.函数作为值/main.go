package main

import "fmt"

func main() {
    // 方法1：字面量初始化（包含函数定义）
    mf := map[int]func() int{
        1: func() int { return 10 },
        2: func() int { return 20 },
        5: func() int { return 50 },
    }
    
    fmt.Printf("Map内容: %v\n", mf) // 输出函数的指针
    a := mf[1]()                   // 调用某个分支的函数
    fmt.Printf("调用 mf[1](): %d\n", a)
    
    // 调用其他分支
    fmt.Printf("调用 mf[2](): %d\n", mf[2]())
    fmt.Printf("调用 mf[5](): %d\n", mf[5]())
}