package main
import (
    "fmt"
    "math/rand"
    "sync"
    "time"
)
type Task struct {
    id      int
    randnum int
}
type Result struct {
    task   Task
    result int
}
var tasks = make(chan Task, 10)
var results = make(chan Result, 10)
func process(num int) int {
    sum := 0
    for num != 0 {
        digit := num % 10
        sum += digit
        num /= 10
    }
    time.Sleep(2 * time.Second)
    return sum
}
func worker(wg *sync.WaitGroup) {
    defer wg.Done()
    for task := range tasks {//从tasks中获取任务并执行,直到任务队列为空且通道关闭，range循环结束。
        result := Result{task, process(task.randnum)}
        results <- result
    }
}
func createWorkerPool(numOfWorkers int) {
    var wg sync.WaitGroup
    for i := 0; i < numOfWorkers; i++ {
        wg.Add(1)
        go worker(&wg)
    }
    wg.Wait()//这里等待所有worker进程结束才关掉results
    close(results)
}
func allocate(numOfTasks int) {
    for i := 0; i < numOfTasks; i++ {
        randnum := rand.Intn(999)
        task := Task{i, randnum}
        tasks <- task
    }
    close(tasks)
}
func getResult(done chan bool) {
    for result := range results {//当通道被关闭（closed）并且通道中所有已发送的数据都被接收完毕后，range循环会自动结束。
        fmt.Printf("Task id %d, randnum %d , sum %d\n", result.task.id, result.task.randnum, result.result)
    }
    done <- true
}
func main() {
    startTime := time.Now()
    numOfWorkers := 20
    numOfTasks := 100
    var done = make(chan bool)
    go getResult(done)
    go allocate(numOfTasks)
    go createWorkerPool(numOfWorkers)
    // 必须在allocate()和getResult()之后创建工作池
    <-done   //解释一下这里的done是怎么起作用的,<-done里面如果去不出来那个true,就会一直塞,然后要拿这个true的话,要对results通道的range要结束,结束的条件就是通道关闭,然而results通道关闭会在createWorkerPool函数中等wg.wait全部回来后也就是线程全部结束后执行close(results)
    endTime := time.Now()
    diff := endTime.Sub(startTime)
    fmt.Println("total time taken ", diff.Seconds(), "seconds")
}