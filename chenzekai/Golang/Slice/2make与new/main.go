package main

import (
	"fmt"
)

//? make和new的区别详解

//! 1. 基本概念对比
//! new(T) - 分配类型T的零值内存，返回*T（指向T的指针）
//! make(T, args) - 仅用于slice、map、channel，初始化并返回T（不是指针）

//! 2. new函数的特点
//! - 适用于所有类型（包括基本类型、结构体、数组等）
//! - 分配内存并将内存初始化为类型的零值
//! - 返回指向该内存的指针（*T类型）
//! - 相当于&T{}的操作

//! 3. make函数的特点  
//! - 仅适用于三种引用类型：slice、map、channel
//! - 不仅分配内存，还进行初始化（不仅仅是零值）
//! - 返回的是类型本身（T类型），不是指针
//! - 对于slice：创建底层数组并返回切片头
//! - 对于map：创建哈希表结构
//! - 对于channel：创建通道缓冲区

func main() {
	//? 示例1: new的使用
	fmt.Println("=== new函数示例 ===")
	
	// new用于基本类型
	intPtr := new(int)
	fmt.Printf("new(int): %v, type: %T, value: %d\n", intPtr, intPtr, *intPtr)
	
	// new用于结构体
	type Person struct {
		Name string
		Age  int
	}
	personPtr := new(Person)
	fmt.Printf("new(Person): %v, type: %T, value: %+v\n", personPtr, personPtr, *personPtr)
	
	// new用于数组
	arrPtr := new([3]int)
	fmt.Printf("new([3]int): %v, type: %T, value: %v\n", arrPtr, arrPtr, *arrPtr)
	
	//? 示例2: make的使用
	fmt.Println("\n=== make函数示例 ===")
	
	// make用于slice
	slice1 := make([]int, 3)        // 长度3，容量3
	slice2 := make([]int, 2, 5)     // 长度2，容量5
	fmt.Printf("make([]int, 3): %v, type: %T, len: %d, cap: %d\n", slice1, slice1, len(slice1), cap(slice1))
	fmt.Printf("make([]int, 2, 5): %v, type: %T, len: %d, cap: %d\n", slice2, slice2, len(slice2), cap(slice2))
	
	// make用于map
	map1 := make(map[string]int)
	map2 := make(map[string]int, 10) // 指定初始容量
	fmt.Printf("make(map[string]int): %v, type: %T\n", map1, map1)
	fmt.Printf("make(map[string]int, 10): %v, type: %T\n", map2, map2)
	
	// make用于channel
	ch1 := make(chan int)      // 无缓冲通道
	ch2 := make(chan int, 5)   // 缓冲大小为5的通道
	fmt.Printf("make(chan int): %v, type: %T\n", ch1, ch1)
	fmt.Printf("make(chan int, 5): %v, type: %T\n", ch2, ch2)
	
	//? 示例3: 错误用法演示
	fmt.Println("\n=== 错误用法演示 ===")
	
	sliceWrong := new([]int)  // 这样创建的是指向nil切片的指针
	println(sliceWrong, *sliceWrong)
	fmt.Printf("错误方式: %v, value: %v\n", sliceWrong, *sliceWrong)
	(*sliceWrong)[0] = 10 // 这会导致运行时错误，因为*sliceWrong是nil，没有底层数组,panic: runtime error: index out of range [0] with length 0
	
	// 正确的做法：
	sliceCorrect := new([]int)  // 创建指向切片的指针
	*sliceCorrect = make([]int, 3) // 然后用make初始化
	println(sliceCorrect, *sliceCorrect)
	fmt.Printf("正确方式: %v, value: %v\n", sliceCorrect, *sliceCorrect)
	

}