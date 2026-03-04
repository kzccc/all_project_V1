	package main
	
	import "fmt"

	func main() {
	
//? ------------------------------------------------------------------------------------------------------------------
		// nil map - 未初始化的map，指向nil
		var nil_map map[string]string
		fmt.Printf("nil_map: %v\n", nil_map)
		fmt.Printf("nil_map地址: %p\n", nil_map)
		fmt.Printf("nil_map是否为nil: %v\n", nil_map == nil)
	
		// empty map - 已初始化但为空的map
		emp_map := map[string]string{}
		fmt.Printf("\nemp_map: %v\n", emp_map)
		fmt.Printf("emp_map地址: %p\n", emp_map)
		fmt.Printf("emp_map是否为nil: %v\n", emp_map == nil)
	
		// 演示两者的行为差异
		fmt.Println("\n=== 操作测试 ===")
	
		//? 尝试向nil map添加元素（会panic）
		/* fmt.Println("尝试向nil_map添加元素:")
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("panic: %v\n", r)
			}
		}()
		nil_map["key"] = "value" */
	
		// 向empty map添加元素（正常工作）
		fmt.Println("\n尝试向emp_map添加元素:")
		emp_map["key"] = "value"
		fmt.Printf("emp_map添加后: %v\n", emp_map)
	
		// 演示长度检查
		fmt.Printf("\nlen(nil_map): %d\n", len(nil_map))
		fmt.Printf("len(emp_map): %d\n", len(emp_map))
	
		// 演示迭代
		fmt.Println("\n=== 迭代测试 ===")
		fmt.Println("遍历nil_map:")
		for k, v := range nil_map {
			fmt.Printf("key: %s, value: %s\n", k, v)
		}
	
		fmt.Println("\n遍历emp_map:")
		for k, v := range emp_map {
			fmt.Printf("key: %s, value: %s\n", k, v)
		}

	
//? ------------------------------------------------------------------------------------------------------------------

		// 验证map类型是引用类型（指针）
		fmt.Println("\n=== 验证map是引用类型 ===")
		// 创建一个map并赋值给另一个变量
		map1 := map[string]int{"a": 1, "b": 2}
		map2 := map1 // 赋值，不是复制
		map2["c"] = 3
	
		fmt.Printf("map1: %v\n", map1)
		fmt.Printf("map2: %v\n", map2)
		// map不能直接比较相等性，只能与nil比较或通过其他方式比较内容
		fmt.Printf("map1和map2内容是否相同: %v\n", mapsEqual(map1, map2))
		fmt.Printf("map1是否为nil: %v\n", map1 == nil)
		fmt.Printf("map2是否为nil: %v\n", map2 == nil)
	
		// 演示map的引用特性
		fmt.Printf("map1和map2是否指向同一底层数据: 是，因为map是引用类型\n")
	}
	
	// mapsEqual 比较两个map的内容是否相等
	func mapsEqual(m1, m2 map[string]int) bool {
		if len(m1) != len(m2) {
			return false
		}
		
		for k, v := range m1 {
			if val, ok := m2[k]; !ok || val != v {
				return false
			}
		}
		return true
	}



