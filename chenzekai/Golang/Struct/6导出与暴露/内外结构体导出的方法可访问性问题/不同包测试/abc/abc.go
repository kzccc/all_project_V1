package abc

import "fmt"

// 未导出的person
type person struct {
    name string
    age  int
}

// 未导出的方法
func (p *person) speak() {
    fmt.Println("speak in person")
}

// 导出的方法
func (p *person) Sing() {
    fmt.Println("Sing in person")
}

// Admin exported
type Admin struct {
    person
    salary int
}

