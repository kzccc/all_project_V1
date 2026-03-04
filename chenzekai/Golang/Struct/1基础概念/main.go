package main

import (
	"fmt"
	"time"
	"unsafe"
)

// sizeof 函数用于计算变量占用的内存大小
func sizeof(v interface{}) uintptr {
	return unsafe.Sizeof(v)
}

// showEmptyStructInfo 显示空结构体内存信息的函数
func showEmptyStructInfo() {
	var emptyStruct struct{}
	fmt.Printf("空结构体实例占用内存大小: %d 字节\n", sizeof(emptyStruct))
}

type Student struct {
	// 基本数据类型
	Name      string  // 字符串类型
	Age       int     // 整数类型
	Height    float64 // 浮点数类型
	Weight    float32 // 32位浮点数
	IsActive  bool    // 布尔类型
	
	// 复合数据类型
	Scores    []int        // 切片类型
	Subjects  [5]string    // 数组类型
	Metadata  map[string]interface{} // 映射类型
	
	// 时间类型
	Birthday  time.Time    // 时间类型
	
	// 指针类型
	Address   *string      // 字符串指针
	ParentAge *int         // 整数指针
	
	// 结构体类型
	Contact   ContactInfo  // 嵌套结构体
	
	// 接口类型
	Data      interface{}  // 空接口类型
	
	// 通道类型
	MessageCh chan string  // 字符串通道
	
	// 函数类型
	Calculator func(int, int) int // 函数类型
}

// 嵌套的联系信息结构体
type ContactInfo struct {
	Email   string
	Phone   string
	Address string
}

// demonstrateZeroValue 展示结构体零值初始化的知识点
func demonstrateZeroValue() {
	fmt.Println("=== 结构体零值初始化演示 ===")
	
	// 创建一个未初始化的Student结构体实例
	var student Student
	
	fmt.Printf("Name: %q (类型: string, 零值: \"\")\n", student.Name)
	fmt.Printf("Age: %d (类型: int, 零값: 0)\n", student.Age)
	fmt.Printf("Height: %.1f (类型: float64, 零값: 0.0)\n", student.Height)
	fmt.Printf("Weight: %.1f (类型: float32, 零값: 0.0)\n", student.Weight)
	fmt.Printf("IsActive: %t (类型: bool, 零값: false)\n", student.IsActive)
	
	fmt.Printf("Scores: %v (类型: []int, 零값: nil)\n", student.Scores)
	fmt.Printf("Subjects: %v (类型: [5]string, 零값: [\"\" \"\" \"\" \"\" \"\"])\n", student.Subjects)
	fmt.Printf("Metadata: %v (类型: map[string]interface{}, 零값: nil)\n", student.Metadata)
	
	fmt.Printf("Birthday: %v (类型: time.Time, 零값: 0001-01-01 00:00:00 +0000 UTC)\n", student.Birthday)
	
	fmt.Printf("Address: %v (类型: *string, 零값: nil)\n", student.Address)
	fmt.Printf("ParentAge: %v (类型: *int, 零값: nil)\n", student.ParentAge)
	
	fmt.Printf("Contact: %+v (类型: ContactInfo, 零값: {Email:\"\" Phone:\"\" Address:\"\"})\n", student.Contact)
	
	fmt.Printf("Data: %v (类型: interface{}, 零값: <nil>)\n", student.Data)
	
	fmt.Printf("MessageCh: %v (类型: chan string, 零값: nil)\n", student.MessageCh)
	
	fmt.Printf("Calculator: %v (类型: func(int, int) int, 零값: nil)\n", student.Calculator)
	
	fmt.Println()
}

// showStudentInfo 展示Student实例的详细信息，接收Student实例作为参数
func showStudentInfo(student Student) {
	fmt.Println("=== 学生信息展示 ===")
	
	fmt.Printf("学生姓名: %s\n", student.Name)
	fmt.Printf("年龄: %d\n", student.Age)
	fmt.Printf("身高: %.1f cm\n", student.Height)
	fmt.Printf("体重: %.1f kg\n", student.Weight)
	fmt.Printf("是否活跃: %t\n", student.IsActive)
	
	fmt.Printf("成绩: %v\n", student.Scores)
	fmt.Printf("科目: %v\n", student.Subjects)
	fmt.Printf("元数据: %v\n", student.Metadata)
	fmt.Printf("生日: %v\n", student.Birthday.Format("2006-01-02"))
	
	if student.Address != nil {
		fmt.Printf("地址: %s\n", *student.Address)
	}
	if student.ParentAge != nil {
		fmt.Printf("父母年龄: %d\n", *student.ParentAge)
	}
	
	fmt.Printf("联系方式 - 邮箱: %s, 电话: %s\n", 
		student.Contact.Email, student.Contact.Phone)
	
	fmt.Printf("计算结果: 10 + 20 = %d\n", student.Calculator(10, 20))
	
	// 演示通道使用
	go func() {
		student.MessageCh <- "Hello, World!"
	}()
	
	message := <-student.MessageCh
	fmt.Printf("收到消息: %s\n", message)
	
	fmt.Println()
}

func main() {
	
	
	// 创建学生实例
	student := Student{
		Name:     "张三",
		Age:      20,
		Height:   175.5,
		Weight:   68.3,
		IsActive: true,
		Scores:   []int{85, 92, 78, 96, 88},
		Subjects: [5]string{"数学", "英语", "物理", "化学", "生物"},
		Metadata: map[string]interface{}{
			"grade":    "大二",
			"major":    "计算机科学",
			"gpa":      3.8,
			"isMember": true,
		},
		Birthday: time.Date(2003, time.May, 15, 0, 0, 0, 0, time.UTC),
		Contact: ContactInfo{
			Email:   "zhangsan@example.com",
			Phone:   "13800138000",
			Address: "北京市海淀区",
		},
		Data:      "额外信息",
		MessageCh: make(chan string, 10),
		Calculator: func(a, b int) int {
			return a + b
		},
	}
	
	// 使用指针类型
	address := "北京市朝阳区"
	parentAge := 45
	student.Address = &address
	student.ParentAge = &parentAge
	//! 调用演示函数
	showStudentInfo(student)



	//!struct初始化时，会做默认的赋0初始化，会给它的每个字段根据它们的数据类型赋予对应的0值。例如int类型是数值0，string类型是""，引用类型是nil等。
	demonstrateZeroValue()

	//!    struct 是 内存布局模板,实际上不占用任何的内存
	//!    实例是 按模板分配出来的一块真实内存 ,空结构体不占用内存
	//!    每个实例都拥有自己的那一块内存，不共享
	
	//!实际上空结构体不占用内存,通常用来忽略某些返回值以及某些import包只为了调用init函数
	showEmptyStructInfo()

	//!但是如果是 u := Student{} 这种情况,虽然没有给字段赋值,但是实例本身会占用内存,因为它有字段模板
	//!其次有两种会分配内存到堆上的写法: u := &Student{} 或者 u := new(Student)
	//!实际上这两种写法都会在堆上分配内存并返回指针主要区别在于：&Student{} 可以同时进行字段初始化，而 new(Student) 只是分配零值内存
	//! var p *Student 这种写法只是声明了一个指针变量,并没有分配内存,所以不能直接使用,需要先分配内存后才能使用
	//! 可以是用 p = &Student{} 或者 p = new(Student) 来分配内存 



	//!匿名结构体通过直接定义无类型名的 struct 并赋给变量，用于一次性或临时数据聚合场景，避免为不可复用的数据结构单独声明类型。
	//!匿名字段用类型名作为字段名来嵌入结构体，从而实现字段与方法的自动提升，达到用组合模拟继承的效果。
}













	