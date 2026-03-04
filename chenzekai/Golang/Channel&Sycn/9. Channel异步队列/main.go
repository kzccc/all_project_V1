package main

import (
    "fmt"
    "math/rand"
    "sync"
    "time"
)

type Task struct {
    ID       int
    JobID    int
    Status   string
    CreateTime time.Time
}

func (t *Task) run() {
    sleep := rand.Intn(1000)
    time.Sleep(time.Duration(sleep) * time.Millisecond)
    t.Status = "Completed"
}

var wg sync.WaitGroup

const workerNum = 3

func main() {
    wg.Add(workerNum)
    
    // 创建容量为10的buffered channel
    taskQueue := make(chan *Task, 10)
    
    // 启动worker goroutine
    for workID := 0; workID < workerNum; workID++ {
        go worker(taskQueue, workID)
    }
    
    // 将任务放入channel
    for i := 1; i <= 15; i++ {
        taskQueue <- &Task{
            ID:       i,
            JobID:    100 + i,
            CreateTime: time.Now(),
        }
    }
    
    close(taskQueue)
    wg.Wait()
}

// 从channel中读取任务并执行
func worker(in <-chan *Task, workID int) {
    defer wg.Done()
    
    for v := range in {
        fmt.Printf("Worker%d: recv a request: TaskID:%d, JobID:%d\n", workID, v.ID, v.JobID)
        v.run()
        fmt.Printf("Worker%d: Completed for TaskID:%d, JobID:%d\n", workID, v.ID, v.JobID)
    }
}