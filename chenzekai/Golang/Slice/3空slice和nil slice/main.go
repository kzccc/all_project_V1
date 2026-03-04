package main

import (
	"fmt"
	"unsafe"
)

func main() {
	// nil slice - 未初始化的slice，指向nil
	var slice1 []int
	
	// empty slice - 初始化但长度为0的slice,指针可能是nil,也可能指向一个零长度的底层数组对象
	slice2 := []int{}
	
	// empty slice - 使用new创建的slice指针,指向一个nil slice,,这个slice头在堆上，但它的内容是一个 nil slice（ptr=nil, len=0, cap=0）
	slice3 := new([]int)
	
	// empty slice - 使用make创建长度为0的slice
	slice4 := make([]int, 0)
	
	// empty slice - 使用make创建长度为4但未初始化值的slice
	slice5 := make([]int, 4)

	fmt.Println("slice1 (nil slice):", slice1)
	fmt.Println("slice2 (empty slice {}):", slice2)
	fmt.Println("slice3 (new([]int)):", slice3)
	fmt.Println("slice4 (make([]int, 0)):", slice4)
	fmt.Println("slice5 (make([]int, 4)):", slice5)
	fmt.Println("---------------------------------------")

	fmt.Println("Size of slice struct:", unsafe.Sizeof(slice1))
	fmt.Printf("slice1 (nil slice): len=%d, cap=%d, is nil: %t\n", len(slice1), cap(slice1), slice1 == nil)
	fmt.Printf("slice2 (empty slice {}): len=%d, cap=%d, is nil: %t\n", len(slice2), cap(slice2), slice2 == nil)
	fmt.Printf("slice3 (new([]int)): len=%d, cap=%d, is nil: %t\n", len(*slice3), cap(*slice3), *slice3 == nil)
	fmt.Printf("slice4 (make([]int, 0)): len=%d, cap=%d, is nil: %t\n", len(slice4), cap(slice4), slice4 == nil)
	fmt.Printf("slice5 (make([]int, 4)): len=%d, cap=%d, is nil: %t\n", len(slice5), cap(slice5), slice5 == nil)

	// slice1 是一个 nil slice，它的 len=0、cap=0，
	// 它本身作为一个 slice 变量占用 24 字节的 slice 头部内存（ptr/len/cap），
	// 但它没有任何底层数组内存，ptr == nil。

	// slice2 是一个非 nil 的空 slice，它的 len=0、cap=0，ptr != nil，
	// 它占用一个 24 字节的 slice 头部内存，
	// 并且 runtime 会为它分配一个零长度的底层数组对象，
	// 因此它确实有底层数组指针和对应的堆对象（即使不存任何元素）。

	// slice3 是一个指向 slice 头的指针（*[]int），这个 slice 头在堆上，
	// 该 slice 头的内容是一个 nil slice（ptr=nil, len=0, cap=0）。
	// 因此它额外占用一个指针（8字节）+ 一个 slice 头（24字节），
	// 但同样没有任何底层数组内存。

	// slice4 是一个非 nil 的空 slice，它的 len=0、cap=0，ptr != nil，
	// 它占用一个 24 字节的 slice 头部内存，
	// 并且 runtime 会为它分配一个零长度的底层数组对象，
	// 因此它确实有底层数组指针和对应的堆对象（即使不存任何元素）。

	// slice5 是一个非 nil 的非空 slice，它的 len=4、cap=4，ptr != nil，
	// 它占用一个 24 字节的 slice 头部内存，
	// 并且 runtime 会为它分配一个包含 4 个 int 元素的底层数组，
	// 因此它真实占用的数据区内存为 4 * sizeof(int)，在 64 位系统上是 32 字节。
}