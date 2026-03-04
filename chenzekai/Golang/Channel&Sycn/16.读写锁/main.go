package main

import (
    "fmt"
    "sync"
    "time"
)

// 模拟一个数据库，包含多个用户信息
type Database struct {
    users  map[string]string // 用户名到密码的映射
    rwmu   sync.RWMutex      // 读写锁
    mu     sync.Mutex        // 普通互斥锁，用于对比
}

func NewDatabase() *Database {
    return &Database{
        users: make(map[string]string),
        rwmu:  sync.RWMutex{},
        mu:    sync.Mutex{},
    }
}

// 使用读写锁的读操作 - 多个goroutine可以同时读取
func (db *Database) ReadWithRWMutex(username string) string {
    db.rwmu.RLock()
    defer db.rwmu.RUnlock()
    fmt.Printf("Reading user %s with RWMutex\n", username)
    time.Sleep(100 * time.Millisecond) // 模拟读取延迟
    return db.users[username]
}

// 使用普通互斥锁的读操作 - 只能有一个goroutine读取
func (db *Database) ReadWithMu(username string) string {
    db.mu.Lock()
    defer db.mu.Unlock()
    fmt.Printf("Reading user %s with Mutex\n", username)
    time.Sleep(100 * time.Millisecond) // 模拟读取延迟
    return db.users[username]
}

// 写操作 - 只能有一个goroutine写入
func (db *Database) Write(username, password string) {
    db.rwmu.Lock()
    defer db.rwmu.Unlock()
    fmt.Printf("Writing user %s with RWMutex\n", username)
    time.Sleep(5000 * time.Millisecond) // 模拟写入延迟
    db.users[username] = password
}

// 添加用户
func (db *Database) AddUser(username, password string) {
    db.rwmu.Lock()
    defer db.rwmu.Unlock()
    db.users[username] = password
    fmt.Printf("Added user %s\n", username)
}

func main() {
    db := NewDatabase()
    
    // 添加一些初始用户
    db.AddUser("user0", "password0")
    db.AddUser("user1", "password1")
    db.AddUser("user2", "password2")
    db.AddUser("user3", "password3")
    db.AddUser("user4", "password4")

    // 创建多个读取goroutine
    var wg sync.WaitGroup
    
    // 启动5个读取goroutine，使用读写锁
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            username := fmt.Sprintf("user%d", id)
            password := db.ReadWithRWMutex(username)
            fmt.Printf("Read result: %s -> %s\n", username, password)
        }(i)
    }

    // 启动一个写入goroutine
    wg.Add(1)
    go func() {
        defer wg.Done()
        db.Write("user0", "new_password")
        fmt.Println("Updated alice's password")
    }()

    // 等待所有goroutine完成
    wg.Wait()
    
    fmt.Println("\n=== 使用普通互斥锁进行对比 ===")
    
    // 重置数据库
    db = NewDatabase()
    // 添加一些初始用户
    db.AddUser("user0", "password0")
    db.AddUser("user1", "password1")
    db.AddUser("user2", "password2")
    db.AddUser("user3", "password3")
    db.AddUser("user4", "password4")

    // 启动5个读取goroutine，使用普通互斥锁
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            username := fmt.Sprintf("user%d", id)
            password := db.ReadWithMu(username)
            fmt.Printf("Read result: %s -> %s\n", username, password)
        }(i)
    }

    // 启动一个写入goroutine,不过这里读写不会阻塞,一个是读锁,一个是写锁
    wg.Add(1)
    go func() {
        defer wg.Done()
        db.Write("user0", "new_password")
        fmt.Println("Updated alice's password")
    }()

    // 等待所有goroutine完成
    wg.Wait()
}