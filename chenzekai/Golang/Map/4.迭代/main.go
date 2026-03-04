package main


import "fmt"

func main() {
    // 创建并初始化map
    my_map := map[string]int{
        "Java":    11,
        "Perl":    8,
        "Python":  13,
        "Shell":   23,
        "Go":      1,  // 添加Go语言
        "Rust":    3,  // 添加Rust语言
    }
    
    // 1. 同时获取键值对
    fmt.Println("=== 1. 同时遍历键值对 ===")
    // 使用range迭代map，返回key和value
    for key, value := range my_map {
        fmt.Printf("key: %s, value: %d\n", key, value)
    }
    
    // 2. 只获取键
    fmt.Println("\n=== 2. 只遍历键 ===")
    // 如果只给一个返回值，则只返回key
    for key := range my_map {
        fmt.Printf("key: %s\n", key)
    }
    
    // 3. 只获取值（使用空白标识符）
    fmt.Println("\n=== 3. 只遍历值 ===")
    // 使用空白标识符_忽略key，只获取value
    for _, value := range my_map {
        fmt.Printf("value: %d\n", value)
    }
    
    // 4. 额外示例：统计信息
    fmt.Println("\n=== 4. Map统计信息 ===")
    fmt.Printf("Map中共有 %d 个元素\n", len(my_map))
    
    // 5. 查找特定键
    fmt.Println("\n=== 5. 查找特定键 ===")
    if value, exists := my_map["Go"]; exists {
        fmt.Printf("找到键 'Go'，值为: %d\n", value)
    } else {
        fmt.Println("未找到键 'Go'")
    }
    
    // 6. 检查不存在的键
    if value, exists := my_map["C++"]; exists {
        fmt.Printf("找到键 'C++'，值为: %d\n", value)
    } else {
        fmt.Println("未找到键 'C++'")
    }
}