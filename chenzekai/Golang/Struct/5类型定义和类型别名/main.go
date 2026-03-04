package main

import "fmt"

// 原始类型
type MyInt int

// 为 MyInt 定义方法
func (m MyInt) Add(x int) MyInt {
    return m + MyInt(x)
}

// 类型定义 - 创建新类型
type NewInt MyInt  // 没有 = 号，创建新类型
// 类型别名 - 同一个类型
type AliasInt = MyInt  // 有 = 号，只是别名

func main() {
    var a MyInt = 10
    fmt.Println(a.Add(5))  // 正常：15
    


    var b NewInt = 20
	var c AliasInt = 30
    // fmt.Println(b.Add(5))  // 错误！NewInt 没有 Add 方法
    fmt.Println(c.Add(5))     // 正确：35



    // 需要类型转换
    fmt.Println(MyInt(b).Add(5))  // 正确：25


    fmt.Println("\n=== 关键区别总结 ===")
    fmt.Println("1. type NewInt MyInt - 新类型定义，不继承方法，有自己的类型身份")
    fmt.Println("2. type AliasInt = MyInt - 类型别名，完全等价于原类型，共享方法")
    fmt.Println("3. NewInt 有自己的方法集，与 MyInt 不同,不继承MyInt的方法")
    fmt.Println("4. AliasInt 与 MyInt 有相同的方法集")
}






