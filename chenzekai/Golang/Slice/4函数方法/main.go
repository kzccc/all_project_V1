package main
import (
	"fmt"
	"unsafe"
)

func main() { 
	//?    copy函数的使用 n = copy(dst, src)  其中n = min(len(dst), len(src)), copy 永远只拷贝 “能放下的那一部分”。
	//?    copy常用于深拷贝
	print("copy函数的使用示例:\n")
	s1 := []int{11, 22, 33}
	s2 := make([]int, 5)
	s3 := make([]int,2)

	num := copy(s2, s1)
	copy(s3,s1)

	fmt.Println(num)  // 3
	fmt.Println(s2)   // [11,22,33,0,0]
	fmt.Println(s3)   // [11,22]
	//!这是深拷贝,因为 copy 函数会将 src 中的元素逐个复制到 dst 中，dst 和 src 之间没有任何共享的底层数组或内存区域。修改 dst 不会影响 src，反之亦然。
	//!需要注意的是，copy 函数的行为是基于元素的复制，而不是引用的复制。对于基本类型（如 int、string 等），copy 会直接复制值；对于引用类型（如 slice、map、channel 等），copy 会复制引用，但不会复制底层数据结构。因此，在使用 copy 进行深拷贝时，需要确保 dst 和 src 之间没有共享的底层数据结构，以避免意外的副作用。
	println(s1)
	println(s2)
	println(s3)
	println()

	
	//?  qppend函数的使用 dst = append(dst, elems...)  把元素追加到 slice 末尾，如果底层数组装不下，就分配一个新的数组并拷贝过去。
	//?  原 slice 还指向旧数组，新 slice 指向新数组
	//?  扩容:当切片的长度等于容量的时候,使用append会自动扩展切片的容量,这时候接收函数的返回值的切片将会是一个新的切片,拥有一个新的底层数组,容量每次一扩展就会扩展两倍
	//?  如果底层数组的长度超过1000时，将按照25%的比率扩容，也就是1000个元素时，将扩展为1250个，不过这个增长比率的算法可能会随着go版本的递进而改变.	
	print("append函数的使用和扩容示例:\n")
	s := make([]int, 0, 2) // len=0, cap=2

	fmt.Println("初始状态")
	printSlice("s", s)

	fmt.Println("\n--- append 1 ---")
	s = append(s, 1)
	printSlice("s", s)

	fmt.Println("\n--- append 2 ---")
	s = append(s, 2)
	printSlice("s", s)

	fmt.Println("\n--- append 3（触发扩容） ---")
	s = append(s, 3)
	printSlice("s", s)

	fmt.Println("\n--- append 4 ---")
	s = append(s, 4)
	printSlice("s", s)
	println()


	//?一次扩容不够的情况
	print("一次扩容不够的情况:\n")
	// 初始 cap = 2
	ss := make([]int, 2, 2)
	ss[0], ss[1] = 1, 2

	fmt.Printf("Before append: len=%d cap=%d ptr=%p\n", len(ss), cap(ss), ss)

	// 一次 append 3 个元素 → newLen = 5
	ss = append(ss, 3, 4, 5)

	fmt.Printf("After append:  len=%d cap=%d ptr=%p\n", len(ss), cap(ss), ss)

	// 验证 cap ≥ 5
	fmt.Println("cap >= len ?", cap(ss) >= len(ss))

	// 验证是否发生了扩容（底层数组地址是否改变）
	fmt.Println("Underlying array changed:", unsafe.Pointer(&ss[0]))
	println()




	//? 如何确保 slice 的完全独立性
	print("确保 slice 的完全独立性:\n")

	// 方法1: 限制容量的切片
	print("\n限制容量的切片示例:\n")
	my_slice := []int{11, 22, 33, 44, 55}
	new_slice := my_slice[2:3:3] // 长度=1,容量=1
	fmt.Printf("my_slice: %v, len=%d, cap=%d\n", my_slice, len(my_slice), cap(my_slice))
	fmt.Printf("new_slice: %v, len=%d, cap=%d\n", new_slice, len(new_slice), cap(new_slice))
	// 任何append操作都会触发扩容
	fmt.Println("对new_slice进行append操作:")
	new_slice = append(new_slice, 99)
	fmt.Printf("new_slice after append: %v, len=%d, cap=%d\n", new_slice, len(new_slice), cap(new_slice))
	fmt.Printf("my_slice after append to new_slice: %v\n", my_slice)

	// 方法2: 零长度零容量切片
	print("\n零长度零容量切片示例:\n")
	independent := my_slice[:0:0] // 长度=0,容量=0
	fmt.Printf("my_slice: %v, len=%d, cap=%d\n", my_slice, len(my_slice), cap(my_slice))
	fmt.Printf("independent: %v, len=%d, cap=%d\n", independent, len(independent), cap(independent))
	// 完全断绝与原数组的联系
	fmt.Println("对independent进行append操作:")
	independent = append(independent, 100, 200)
	fmt.Printf("independent after append: %v, len=%d, cap=%d\n", independent, len(independent), cap(independent))
	fmt.Printf("my_slice after append to independent: %v\n", my_slice)

	







}

func printSlice(name string, s []int) {
	var ptr uintptr
	if len(s) > 0 {
		ptr = uintptr(unsafe.Pointer(&s[0]))
	}
	fmt.Printf(
		"%s: len=%d cap=%d data_ptr=%x data=%v\n",
		name,
		len(s),
		cap(s),
		ptr,
		s,
	)
}









