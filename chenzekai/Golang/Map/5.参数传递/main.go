package main


import "fmt"

func main() {
    // Map 测试
    m := map[string]int{"a": 1, "b": 2}
    fmt.Println("原始 map:", m)
    
    modifyMapElement(m)
    fmt.Println("1) after modifyMapElement:", m)  // ✅ 修改可见
    
    addToMap(m)
    fmt.Println("2) after addToMap:", m)          // ✅ 新增元素可见
    
    deleteFromMap(m)
    fmt.Println("3) after deleteFromMap:", m)     // ✅ 删除元素可见
    
    // 注意：map 没有"扩容导致新对象"的问题
    m = reassignMap(m)
    fmt.Println("4) after reassignMap:", m)       // 需要赋值才能看到新 map
}

// ① 修改 map 元素 - 外部可见
func modifyMapElement(m map[string]int) {
    m["a"] = 100  // ✅ 直接修改，外部能看到
}

// ② 添加新元素 - 外部可见
func addToMap(m map[string]int) {
    m["c"] = 300  // ✅ 添加新键值对，外部能看到
}

// ③ 删除元素 - 外部可见
func deleteFromMap(m map[string]int) {
    delete(m, "b")  // ✅ 删除操作，外部能看到
}

// ④ 重新赋值 - 需要返回值
func reassignMap(m map[string]int) map[string]int {
    // 创建新 map
    newMap := map[string]int{"x": 1, "y": 2}
    // m = newMap  // ❌ 这样只修改局部变量，外部看不到
    return newMap  // ✅ 返回新 map，让调用方赋值
}