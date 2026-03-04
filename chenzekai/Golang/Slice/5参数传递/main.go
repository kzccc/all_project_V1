package main

import "fmt"
//?  ① s[i] = x         → 只改数组，所有 slice 视图都会看到变化
//?  ② s = append(...) → 只改本函数的 slice 头，外部看不到
//?  ③ return append   → 把新的 slice 头交给外部
//?  ④ *ps = append    → 直接修改外部 slice 头（真正就地扩容）

func main() {
	s := []int{1, 2, 3}

	fmt.Println("原始 s:", s)

	modifyElement(s)
	fmt.Println("1) after modifyElement:", s)

	appendLocal(s)
	fmt.Println("2) after appendLocal:", s)

	s = appendReturn(s)
	fmt.Println("3) after appendReturn:", s)

	appendByPtr(&s)
	fmt.Println("4) after appendByPtr:", s)
}

// ① s[i] = x —— 修改的是底层数组，外部一定可见
func modifyElement(s []int) {
	s[0] = 100
}

// ② s = append(s, x) —— 发生扩容的话只修改了函数内的 slice 头，外部看不到
func appendLocal(s []int) {
	s = append(s, 200)
}

// ③ return append(s, x) —— 把新的 slice 头返回给调用方
func appendReturn(s []int) []int {
	return append(s, 300)
}

// ④ *ps = append(*ps, x) —— 通过指针直接修改外部 slice 头
func appendByPtr(ps *[]int) {
	*ps = append(*ps, 400)
}

