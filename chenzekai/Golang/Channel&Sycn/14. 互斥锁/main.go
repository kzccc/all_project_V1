package main
import (
    "fmt"
    "sync"
    "time"
)
// 共享变量
var (
    m  sync.Mutex
    v1 int
)
// 修改共享变量
// 在Lock()和Unlock()之间的代码部分是临界区
func change(i int) {
    m.Lock()
    time.Sleep(time.Second)
    v1 = v1 + 1
    if v1%10 == 0 {
        v1 = v1 - 10*i
    }
    m.Unlock()
}
// 访问共享变量
// 在Lock()和Unlock()之间的代码部分是是临界区
func read() int {
    m.Lock()
    a := v1
    m.Unlock()
    return a
}
func main() {
    var numGR = 21
    var wg sync.WaitGroup
    fmt.Printf("%d", read())
    // 循环创建numGR个goroutine
    // 每个goroutine都执行change()、read()
    // 每个change()和read()都会持有锁
    for i := 0; i < numGR; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            change(i)
            fmt.Printf(" -> %d", read())
        }(i)
    }
    wg.Wait()
}
//? 对于前9个被调度到的goroutine，无论是哪个goroutine取得这9个change(i)中的critical section，都只是对共享变量v1做加1运算，但当第10个goroutine被调度时，由于v1加1之后得到10，它满足if条件，会执行v1 = v1 - i*10，但这个i可能是任意0到numGR之间的值(因为无法保证并发的goroutine的调度顺序)，这使得v1的值从第10个goroutine开始出现随机性。但从第10到第19个goroutine被调度的过程中，也只是对共享变量v1做加1运算，这些值是可以根据第10个数推断出来的，到第20个goroutine，又再次随机。依此类推。
//? 此外，每个goroutine中的read()也都会参与锁竞争，所以并不能保证每次change(i)之后会随之执行到read()，可能goroutine 1的change()执行完后，会跳转到goroutine 3的change()上，这样一来，goroutine 1的read()就无法读取到goroutine 1所修改的v1值，而是访问到其它goroutine中修改后的值。
//? 所以，前面的第二次执行结果中出现了一次数据跨越(72->74)。只不过执行完change()后立即执行read()的几率比较大，所以多数时候输出的数据都是连续的。
//? 总而言之，Mutex保证了每个critical section安全，某一时间点只有一个goroutine访问到这部分，但也因此而出现了随机性。


