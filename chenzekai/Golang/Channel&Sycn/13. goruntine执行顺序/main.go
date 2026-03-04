//?通过关闭通道后可读这个特性，可以实现一个goroutine执行顺序的技巧。
//?如果一个goroutine A依赖于另一个goroutine B，在goroutine A中首先通过读goroutine B来阻塞自己，直到goroutine B关闭自身之后，goroutine A才会继续运行。这样，goroutine B就先于goroutine A运行。
package main

import (
	"fmt"
	"time"
)

// A首先被a阻塞，A()结束后关闭b，使b可读
// A函数依赖于a通道被关闭才能开始执行，执行完毕后关闭b通道通知其他等待b的goroutine
func A(a, b chan struct{}) {
	<-a          // 阻塞直到a通道被关闭（收到零值）
	fmt.Println("A()!") // A执行
	time.Sleep(time.Second) // 模拟A执行一些耗时操作
	close(b)     // 关闭b通道，通知等待b的goroutine可以继续执行
}

// B首先被a阻塞，B()结束后关闭b，使b可读
// B函数依赖于a通道被关闭才能开始执行，执行完毕后关闭b通道通知其他等待b的goroutine
func B(a, b chan struct{}) {
	<-a          // 阻塞直到a通道被关闭（收到零值）
	fmt.Println("B()!") // B执行
	close(b)     // 关闭b通道，通知等待b的goroutine可以继续执行
}

// C首先被a阻塞
// C函数依赖于a通道被关闭才能开始执行，执行完毕后直接返回
func C(a chan struct{}) {
	<-a          // 阻塞直到a通道被关闭（收到零值）
	fmt.Println("C()!") // C执行
}

func main() {
	x := make(chan struct{}) // x用于控制A的开始执行
	y := make(chan struct{}) // y用于控制B的开始执行，同时也是A执行完毕的通知
	z := make(chan struct{}) // z用于控制三个C和B的开始执行，同时也是B执行完毕的通知

	// 启动三个C是为了演示当z通道关闭后，所有等待z通道的goroutine都会被唤醒
	// 这展示了通道通知的广播特性：一个通道关闭会解除所有等待该通道的goroutine的阻塞状态
	go C(z)  // 启动第一个C，等待z通道关闭才执行
	go A(x, y) // 启动A，等待x通道关闭才执行，执行完毕后关闭y
	go C(z)  // 启动第二个C，等待z通道关闭才执行
	go B(y, z) // 启动B，等待y通道关闭才执行，执行完毕后关闭z
	go C(z)  // 启动第三个C，等待z通道关闭才执行
	
	// 关闭x，让x可读
	close(x)         // 开始执行序列：关闭x允许A开始执行
	time.Sleep(3 * time.Second) // 等待所有goroutine执行完毕
}