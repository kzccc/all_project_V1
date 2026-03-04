package main
import (
    "fmt"
)
type person struct {
    name string
    age  int
}
// 未导出方法
func (p *person) speak() {
    fmt.Println("speak in person")
}
// 导出的方法
func (p *person) Sing() {
    fmt.Println("Sing in person")
}
// Admin exported
type Admin struct {
    p person
    salary int
}
func main() {
    a := new(Admin)
    a.p.speak()  // 正常输出
    a.p.Sing()   // 正常输出
}