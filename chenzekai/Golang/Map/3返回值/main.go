package main

import "fmt"

func main() {
	// 定义一个 map
	myMap := map[string]int{
		"apple":  10,
		"banana": 20,
	}

	// -------- 第一种：只取 value --------
	value1 := myMap["apple"]
	fmt.Println("value1:", value1) // 10

	// key 不存在时，返回该类型的零值
	value2 := myMap["orange"]
	fmt.Println("value2:", value2) // 0（int 的零值）

	// -------- 第二种：取 value + 是否存在 --------
	value3, exists1 := myMap["banana"]
	fmt.Println("value3:", value3, "exists1:", exists1) // 20 true

	value4, exists2 := myMap["orange"]
	fmt.Println("value4:", value4, "exists2:", exists2) // 0 false
}

