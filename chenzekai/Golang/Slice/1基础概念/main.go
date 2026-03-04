package main
import (
	"fmt"
)

func main() { 
//? slice的基础概念
//! slice自身维护了一个指针属性，指向它底层数组中的某些元素的集合。
//! 每一个slice结构都由3部分组成：容量(capacity)、长度(length)和指向底层数组某元素的指针，
//!它们各占8字节(1个机器字长，64位机器上一个机器字长为64bit，共8字节大小，32位架构则是32bit，占用4字节)，所以任何一个slice都是24字节(3个机器字长)。

//? slice的声明和初始化

	//? 1. 使用make函数创建slice
	// make([]T, length) - 创建指定长度的slice，所有元素被初始化为零值
	slice1 := make([]int, 5) // 长度为5，所有元素为0
	fmt.Printf("slice1: %v, len: %d, cap: %d\n", slice1, len(slice1), cap(slice1))

	// make([]T, length, capacity) - 创建指定长度和容量的slice
	slice2 := make([]int, 3, 10) // 长度为3，容量为10
	fmt.Printf("slice2: %v, len: %d, cap: %d\n\n", slice2, len(slice2), cap(slice2))

	//? 3. 通过数组创建slice
	arr := [5]int{1, 2, 3, 4, 5}
	fmt.Printf("arr: %v, len: %d, cap: %d\n", arr, len(arr), cap(arr)) // 显示原数组信息
	slice3 := arr[1:3] // 从数组arr的索引1到2（不包含3），即元素2,3
	fmt.Printf("slice3: %v, len: %d, cap: %d\n\n", slice3, len(slice3), cap(slice3))
	//!切片的容量计算公式是：cap = 底层数组总长度 - 切片起始位置在底层数组中的索引


	//? 4. 直接初始化slice字面量
	slice4 := []int{1, 2, 3, 4, 5}
	fmt.Printf("slice4: %v, len: %d, cap: %d\n\n", slice4, len(slice4), cap(slice4))
	//! 直接初始化的slice字面量会创建一个底层数组，并将slice指向它，长度和容量都等于元素数量。


	//? 5. 声明但不初始化（零值为nil）
	var slice5 []int
	
	fmt.Printf("slice5 (nil slice): %v, len: %d, cap: %d, is nil: %t\n\n", slice5, len(slice5), cap(slice5), slice5 == nil)
	//!未初始化的slice（即零值为nil的slice）没有底层数组，因此它的长度和容量都是0，并且它不占用任何内存空间。只有当你通过make函数、数组切片或直接初始化来创建slice时，才会分配内存并创建底层数组。


	//? 6. 从另一个slice创建新的slice（切片操作）
	slice6 := slice4[1:3]
	fmt.Printf("slice6 (from slice4[1:3]): %v, len: %d, cap: %d\n\n", slice6, len(slice6), cap(slice6))
	//!切片的容量计算公式是：cap = 底层数组总长度 - 切片起始位置在底层数组中的索引


	//? 7. 使用append函数创建或扩展slice
	var slice7 []int
	fmt.Printf("Before append - slice7: %v, len: %d, cap: %d\n", slice7, len(slice7), cap(slice7))
	slice7 = append(slice7, 1)
	fmt.Printf("After append - slice7: %v, len: %d, cap: %d\n\n", slice7, len(slice7), cap(slice7))

	slice7_2 := []int{1,2,3,4,5}
	fmt.Printf("Before append - slice7: %v, len: %d, cap: %d\n", slice7_2, len(slice7_2), cap(slice7_2))
	slice7_2 = append(slice7_2, 1, 2, 3)
	fmt.Printf("After append - slice7: %v, len: %d, cap: %d\n\n", slice7_2, len(slice7_2), cap(slice7_2))
	//!容量不足（需要重新分配内存）时，Go会分配一个更大的底层数组（通常是当前容量的两倍），
	//!slice7一开始len和cap都是0，append多少len和cap就增加多少


	//? 8. 复制slice
	slice8 := make([]int, len(slice4))
	copy(slice8, slice4) // 将slice4的内容复制到slice8
	fmt.Printf("slice8 (copied from slice4): %v, len: %d, cap: %d\n\n", slice8, len(slice8), cap(slice8))
	//!copy函数会复制slice4的内容到slice8，并返回复制的元素数量。len和cap都一致不变
	
	
	//? 9.补充：通过切片操作影响容量的例子(切片的容量计算公式是：cap = 底层数组总长度 - 切片起始位置在底层数组中的索引)
	original := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	fmt.Printf("original: %v, len: %d, cap: %d\n", original, len(original), cap(original))
	subSlice := original[2:5] // 从索引2到4
	fmt.Printf("subSlice (original[2:5]): %v, len: %d, cap: %d\n", subSlice, len(subSlice), cap(subSlice))
	
	// 对于数组创建的切片，容量是从切片起始位置到数组末尾
	arr2 := [10]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	fmt.Printf("arr2: %v, len: %d, cap: %d\n", arr2, len(arr2), cap(arr2))
	slice9 := arr2[3:7] // 从索引3到6
	fmt.Printf("slice9 (arr2[3:7]): %v, len: %d, cap: %d\n", slice9, len(slice9), cap(slice9))
	
	
}









