package main

import "fmt"

func main() {
	// 1. 声明但不初始化（零值为nil）
	var m1 map[string]int
	fmt.Printf("m1: %v, 零值为nil: %v\n", m1, m1 == nil)

	// 2. 使用make函数创建map
	m2 := make(map[string]int)
	fmt.Printf("m2: %v, len: %d\n", m2, len(m2))

	// 3. 创建并指定初始容量
	m3 := make(map[string]int, 10) // 容量为10，但这只是提示，并非固定大小
	fmt.Printf("m3: %v, len: %d\n", m3, len(m3))

	// 4. 直接初始化并赋值（字面值方式）
	m4 := map[string]int{
		"apple":  5,
		"banana": 3,
		"orange": 8,
	}
	fmt.Printf("m4: %v\n", m4)

	// 5. 先声明再赋值
	var m5 map[string]int
	m5 = map[string]int{
		"one": 1,
		"two": 2,
	}
	fmt.Printf("m5: %v\n", m5)

	// 6. 使用make后添加元素
	m6 := make(map[string]int)
	m6["first"] = 1
	m6["second"] = 2
	fmt.Printf("m6: %v\n", m6)

	// 演示各种类型的key和value
	// 字符串到字符串映射
	strToStr := map[string]string{
		"name":    "张三",
		"address": "北京",
	}

	// 整数到切片映射
	intToSlice := map[int][]string{
		1: {"a", "b"},
		2: {"c", "d", "e"},
	}

	// 布尔到整数映射
	boolToInt := map[bool]int{
		true:  1,
		false: 0,
	}

	fmt.Printf("strToStr: %v\n", strToStr)
	fmt.Printf("intToSlice: %v\n", intToSlice)
	fmt.Printf("boolToInt: %v\n", boolToInt)

	// 演示如何安全地访问map中的值
	if val, ok := m4["apple"]; ok {
		fmt.Printf("找到了apple的值: %d\n", val)
	} else {
		fmt.Println("没有找到apple")
	}

	//统计元素
	ans := len(m4)
	println("m4的元素数量:", ans)


	// 删除元素
	delete(m4, "banana")
	fmt.Printf("删除banana后的m4: %v\n", m4)
}