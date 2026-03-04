package main

import (
	"fmt"
)

// Student 结构体定义
type Student struct {
	Name  string
	Age   int
	Score float64
}

// === Go语言构造器模式详解 ===
//
// Go语言没有传统面向对象语言中的构造函数概念，
// 但可以通过函数返回结构体实例的方式来实现类似构造器的功能。
//
// 主要有以下几种构造方式：

// 1. 基础构造函数（推荐方式）
// 返回指针类型，避免值拷贝，且符合Go的惯用法
func NewStudent(name string, age int, score float64) *Student {
	return &Student{
		Name:  name,
		Age:   age,
		Score: score,
	}
}

// 2. 带默认值的构造函数
// 提供更灵活的参数设置
func NewStudentWithDefault(name string, age ...int) *Student {
	s := &Student{
		Name: name,
	}
	
	// 如果提供了age参数，使用提供的值；否则使用默认值
	if len(age) > 0 {
		s.Age = age[0]
	} else {
		s.Age = 18 // 默认年龄
	}
	
	s.Score = 0.0 // 默认分数
	return s
}

// 3. 选项模式构造器（高级用法）
// 支持可选参数，代码更清晰
type StudentOption func(*Student)
//这是一个函数类型别名，定义了一个接受 *Student 参数、返回 void 的函数类型
//本质上是"函数签名"的类型化表示，让代码更清晰

func WithAge(age int) StudentOption {
	return func(s *Student) {
		s.Age = age
	}
}

func WithScore(score float64) StudentOption {
	return func(s *Student) {
		s.Score = score
	}
}

func NewStudentWithOptions(name string, opts ...StudentOption) *Student {
	s := &Student{Name: name}
	
	// 应用所有选项
	for _, opt := range opts {
		opt(s)
	}
	
	return s
}

// 4. 工厂方法模式
// 根据不同条件创建不同类型的实例
func CreateStudent(name string, studentType string) *Student {
	switch studentType {
	case "excellent":
		return &Student{Name: name, Age: 18, Score: 95.0}
	case "average":
		return &Student{Name: name, Age: 18, Score: 75.0}
	default:
		return &Student{Name: name, Age: 18, Score: 60.0}
	}
}

// 5. 非引用类型返回（值类型）
// 返回结构体值而非指针，适用于小结构体
func NewStudentValue(name string, age int, score float64) Student {
	return Student{
		Name:  name,
		Age:   age,
		Score: score,
	}
}

func main() {
	fmt.Println("=== Go语言构造器模式示例 ===\n")

	// 示例1: 基础构造函数
	fmt.Println("1. 基础构造函数:")
	student1 := NewStudent("张三", 20, 85.5)
	fmt.Printf("学生1: %+v\n", student1)

	// 示例2: 带默认值的构造函数
	fmt.Println("\n2. 带默认值的构造函数:")
	student2 := NewStudentWithDefault("李四")
	fmt.Printf("学生2: %+v\n", student2)
	
	student3 := NewStudentWithDefault("王五", 22)
	fmt.Printf("学生3: %+v\n", student3)

	// 示例3: 选项模式构造器
	fmt.Println("\n3. 选项模式构造器:")
	student4 := NewStudentWithOptions("赵六", WithAge(21), WithScore(90.0))
	fmt.Printf("学生4: %+v\n", student4)

	// 示例4: 工厂方法模式
	fmt.Println("\n4. 工厂方法模式:")
	excellentStudent := CreateStudent("钱七", "excellent")
	averageStudent := CreateStudent("孙八", "average")
	fmt.Printf("优秀学生: %+v\n", excellentStudent)
	fmt.Printf("普通学生: %+v\n", averageStudent)

	// 示例5: 非引用类型返回
	fmt.Println("\n5. 非引用类型返回:")
	student5 := NewStudentValue("周九", 19, 88.0)
	fmt.Printf("学生5: %+v\n", student5)

	// === 关键知识点总结 ===
	fmt.Println("\n=== 关键知识点总结 ===")
	fmt.Println("- Go没有构造函数，但可以通过函数返回结构体实例实现构造功能")
	fmt.Println("- 推荐返回指针类型(&Student{})，避免值拷贝，提高性能")
	fmt.Println("- new(Student) 和 &Student{} 都可以创建零值实例，但前者返回指针")
	fmt.Println("- 选项模式适合处理大量可选参数的场景")
	fmt.Println("- 工厂方法适合根据条件创建不同类型的实例")
	fmt.Println("- 对于小结构体，返回值类型可能更高效")
}