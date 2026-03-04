package main

import (
	"fmt"
)
//!----------------------------------------------------------------------------------------------------
type inner struct {
	in1 int
	in2 int
}

type outer struct {
	ou1 int
	ou2 int
	int // 匿名字段（嵌入字段）
	inner
}

func (i *inner) print_in()  {
	fmt.Println(i.in1, i.in2)
	//fmt.Println(i.ou1)//通过内层的方法访问外层的字段是不行的
}
//!----------------------------------------------------------------------------------------------------
//!上面那种方式是匿名结构体嵌套,这种是具名结构体嵌套
// 定义基础结构体
type animal struct {
	name string
	age  int
}

// 具名嵌套：Horse结构体包含一个命名的animal字段
type Horse struct {
	a     animal // 命名的嵌套结构体
	sound string
}

// 为animal结构体添加方法
func (a *animal) introduce() {
	fmt.Printf("我是%s，今年%d岁\n", a.name, a.age)
}

// 为Horse结构体添加方法
func (h *Horse) makeSound() {
	fmt.Printf("%s发出声音：%s\n", h.a.name, h.sound)
}

func (h *Horse) String() string {
	return fmt.Sprintf("马: %s, 年龄: %d, 声音: %s", h.a.name, h.a.age, h.sound)
}
//!----------------------------------------------------------------------------------------------------




func main() {
	//!---------------------------------------------------------------------------------------------------
	fmt.Println("----------------------------------------------------------------------------------------")
	fmt.Println("=== Go语言组合模式示例 ===\n")
	fmt.Println(" 组合是Go语言中实现代码复用的核心机制之一，它允许一个结构体包含另一个结构体作为其字段。这与继承不同，组合更强调“拥有”关系而非“是”关系。")
	o := &outer{
    ou1: 1,
    ou2: 2,
    int: 3,
    inner: inner{
        in1: 4,
        in2: 5,
    },
}
	//另一种申明方式
	/* o := new(outer)
	o.ou1 = 1
	o.ou2 = 2
	o.int = 3
	o.in1 = 4
	o.in2 = 5 */

	fmt.Println(o.ou1) // 1
	fmt.Println(o.ou2) // 2
	fmt.Println(o.int) // 3
	fmt.Println(o.in1) // 4
	fmt.Println(o.in2) // 5
	o.print_in()
	//!--------------------------------------------------------------------------------------------------------------------
	fmt.Println("----------------------------------------------------------------------------------------")
		fmt.Println("=== Go语言具名嵌套结构体示例 ===\n")
	
	// 创建Horse实例
	h := &Horse{
		a: animal{
			name: "小马",
			age:  3,
		},
		sound: "嘶嘶",
	}
	
	// 访问方式演示
	fmt.Println("访问方式：")
	fmt.Printf("动物名字：%s\n", h.a.name)  // 必须通过h.a.name访问
	fmt.Printf("动物年龄：%d\n", h.a.age)   // 必须通过h.a.age访问
	fmt.Printf("马的声音：%s\n", h.sound)   // 直接访问
	
	// 调用方法
	fmt.Println("\n调用方法：")
	h.a.introduce() // 调用animal的方法
	h.makeSound()   // 调用Horse的方法
	
	// 尝试直接访问会报错（注释掉以避免编译错误）
	// fmt.Println(h.name) // 编译错误：找不到name字段

	//!--------------------------------------------------------------------------------------------------------------------
	fmt.Println("----------------------------------------------------------------------------------------")
	fmt.Println("=== 名称冲突问题 ===")
	println("外部struct覆盖内部struct的同名字段、同名方法")
	println("同级别的struct出现同名字段、方法将报错")
	println("")
	println("第一个规则使得Go struct能够实现面向对象中的重写(override)，而且可以重写字段、重写方法。")
	println("")
	println("第二个规则使得同名属性不会出现歧义。例如：")
	println("")
	println("type A struct {")
	println("    a int")
	println("    b int")
	println("}")
	println("")
	println("type B struct {")
	println("    b float32")
	println("    c string")
	println("    d string")
	println("}")
	println("")
	println("type C struct {")
	println("    A")
	println("    B")
	println("    a string")
	println("    c string")
	println("}")
	println("")
	println("var c C")
	println("")
	println("按照规则(1)，直属于C的a和c会分别覆盖A.a和B.c。可以直接使用c.a、c.c分别访问直属于C中的")
	println("a、c字段，使用c.d或c.B.d都访问属于嵌套的B.d字段。如果想要访问内部struct中被覆盖的属性，")
	println("可以c.A.a的方式访问。")
	println("")
	println("按照规则(2)，A和B在C中是同级别的嵌套结构，所以A.b和B.b是冲突的，将会报错，因为当调用")
	println("c.b的时候不知道调用的是c.A.b还是c.B.b。")
	println("\n")
	//!----------------------------------------------------------------------------------------------------
	fmt.Println("----------------------------------------------------------------------------------------")
	fmt.Println("=== 重写string方法 ===")
	//!具体看func (h *Horse) String() string 
	h2 := &Horse{
		a: animal{
			name: "大马",
			age:  5,
		},
		sound: "啊啊",
	}
	// 现在可以直接打印h2，会调用String()方法
	fmt.Println("直接打印h2:", h2)
	// 或者显式调用
	fmt.Println("显式调用String():", h2.String())
	//!----------------------------------------------------------------------------------------------------
	fmt.Println("----------------------------------------------------------------------------------------")
	fmt.Println("=== Go语言不支持函数重载 ===")
	fmt.Println("Go语言不支持传统意义上的函数重载。这意味着你不能定义两个同名但参数不同的函数。")
	fmt.Println("以下是一个示例，展示Go为什么不支持函数重载:")
	fmt.Println("")
	fmt.Println("// 这样的代码会导致编译错误:")
	fmt.Println("// func Add(a int, b int) int {") 
	fmt.Println("//     return a + b")
	fmt.Println("// }")
	fmt.Println("// ")
	fmt.Println("// func Add(a float64, b float64) float64 {")  // 编译错误！
	fmt.Println("//     return a + b")
	fmt.Println("// }")
	fmt.Println("")
	fmt.Println("如果你尝试定义两个同名函数，即使参数类型不同，Go编译器也会报错。")
	fmt.Println("在Go中，如果需要类似重载的功能，通常使用以下几种方式:")
	fmt.Println("1. 使用不同的函数名，如 AddInt, AddFloat64")
	fmt.Println("2. 使用接口和多态*****")
	fmt.Println("3. 使用可变参数函数")
	fmt.Println("4. 使用空接口 interface{} 并配合类型断言")
	
	// 示例：使用不同名称的函数来模拟重载效果
	result1 := addInt(5, 3)
	result2 := addFloat(5.5, 3.2)
	fmt.Printf("整数相加: %d\n", result1)
	fmt.Printf("浮点数相加: %.2f\n", result2)
	
	// 示例：使用可变参数函数
	fmt.Printf("两个整数相加: %d\n", add(1, 2))
	fmt.Printf("三个整数相加: %d\n", add(1, 2, 3))
	fmt.Printf("四个整数相加: %d\n", add(1, 2, 3, 4))
}






// 模拟重载：使用不同函数名
func addInt(a, b int) int {
	return a + b
}

func addFloat(a, b float64) float64 {
	return a + b
}

// 使用可变参数实现类似重载的效果
func add(numbers ...int) int {
	sum := 0
	for _, num := range numbers {
		sum += num
	}
	return sum
}















