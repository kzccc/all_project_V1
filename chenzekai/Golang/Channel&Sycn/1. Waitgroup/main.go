package main
import (  
    "fmt"
    "sync"
    "time"
)
//?还有一点需要特别注意的是process()中使用指针类型的*sync.WaitGroup作为参数，这里不能使用值类型的sync.WaitGroup作为参数
func process(i int, wg *sync.WaitGroup) {  
    fmt.Println("started Goroutine ", i)
    time.Sleep(2 * time.Second)
    fmt.Printf("Goroutine %d ended\n", i)
    wg.Done()
}
func main() {  
    no := 3
    var wg sync.WaitGroup
    for i := 0; i < no; i++ {
        wg.Add(1)
        go process(i, &wg)
    }
    wg.Wait()
    fmt.Println("All go routines finished executing")
}