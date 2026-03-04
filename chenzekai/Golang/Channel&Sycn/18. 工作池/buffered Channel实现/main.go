package main

import (
    "fmt"
    "sync"
    "time"
)

// Task 定义任务结构体
type Task struct {
    id      int
    randnum int
}

// Queue 定义任务队列，使用互斥锁保护
type Queue struct {
    M     sync.Mutex
    Tasks []Task
}

// Worker 工作协程函数，从队列中取出任务并处理
func Worker(queue *Queue) {
    for {
        queue.M.Lock()
        // 检查队列是否为空
        if len(queue.Tasks) == 0 {
            queue.M.Unlock()
            // 如果队列为空，短暂休眠后重试
            time.Sleep(10 * time.Millisecond)
            continue
        }
        
        // 取出第一个任务
        task := queue.Tasks[0]
        
        // 更新任务队列，移除已取出的任务
        queue.Tasks = queue.Tasks[1:]
        
        queue.M.Unlock()
        
        // 在当前goroutine中执行任务
        fmt.Printf("Worker processing task ID: %d, randnum: %d\n", task.id, task.randnum)
        process(task.randnum)
    }
}

// process 模拟任务处理过程
func process(num int) {
    sum := 0
    for num != 0 {
        digit := num % 10
        sum += digit
        num /= 10
    }
    // 模拟耗时操作
    time.Sleep(2 * time.Second)
    fmt.Printf("Task completed, sum: %d\n", sum)
}

// AddTask 向任务队列添加新任务
func (q *Queue) AddTask(task Task) {
    q.M.Lock()
    defer q.M.Unlock()
    q.Tasks = append(q.Tasks, task)
}

// GetTaskCount 获取任务队列中的任务数量
func (q *Queue) GetTaskCount() int {
    q.M.Lock()
    defer q.M.Unlock()
    return len(q.Tasks)
}

func main() {
    startTime := time.Now()
    
    // 创建任务队列
    queue := &Queue{}
    
    // 创建100个任务
    var tasks []Task
    for i := 0; i < 100; i++ {
        randnum := 1 + i*10 // 生成一些测试数据
        tasks = append(tasks, Task{i, randnum})
    }
    
    // 启动10个工作协程
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            Worker(queue)
        }(i)
    }
    
    // 将所有任务添加到队列中
    for _, task := range tasks {
        queue.AddTask(task)
        fmt.Printf("Added task ID: %d to queue\n", task.id)
    }
    
    // 等待所有任务完成
    // 这里需要额外的机制来知道所有任务都已完成
    // 由于Worker是无限循环的，我们需要在适当的时候停止它们
    
    // 让程序运行一段时间后退出
    time.Sleep(30 * time.Second)
    
    // 停止工作协程（这里简化处理）
    fmt.Println("Stopping workers...")
    wg.Wait()
    
    endTime := time.Now()
    diff := endTime.Sub(startTime)
    fmt.Println("Total time taken:", diff.Seconds(), "seconds")
}
