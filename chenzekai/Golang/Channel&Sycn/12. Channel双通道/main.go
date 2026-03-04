package main

import (
    "fmt"
    "time"
)

//?不建议理解,看看就行,感觉硬写的逻辑,很复杂




//?定义一个双层通道,然后循环五次,每一次调用f1()
//? f1()需要传入双层通道cc和信号通道f
//? 在f1内部再创建一个内层通道c
func main() {
    // 定义双层通道cc
    cc := make(chan chan int)
    // 定义信号通道f
    f := make(chan bool)

    // 只启动一次f1()
    go f1(cc, f)

    times := 5
    for i := 1; i < times+1; i++ {
        // 从双层通道cc中取出内层通道ch
        // 并向ch通道发送数据
        ch := <-cc//双层通道有值是因为f1()启动生成了内层通道c
        ch <- i//拿出来再放置数值int进去

        // 从ch中取出数据
		//其实就是ch一旦有数据也就是f1一旦生成了数据放进来,就会被取出来打印
        fmt.Printf("Sum(%d)=%d\n", i, <-ch) // 直接接收数据而不是使用range循环
    }
    
    // 最后关闭信号通道f
    close(f)
    
    // 给一点时间让f1协程处理关闭信号
    time.Sleep(time.Millisecond * 100)
}

// 双层通道cc用来生成内层通道c
// 并使用信号通道f来终止函数f1()
func f1(cc chan chan int, f chan bool) {
    for {
        c := make(chan int)
        select {
        case cc <- c: // 将新创建的通道发送到双层通道
            // 从内层通道中取出数据，计算和，然后发回内层通道
            x := <-c
            sum := 0
            for i := 0; i <= x; i++ {
                sum = sum + i
            }
            // goroutine将阻塞在此，直到数据被读走
            c <- sum
        case <-f: // 信号通道f可读时，结束f1()的运行
            return
        }
    }
}