package main
import "fmt"
type person struct {
    name string
    age  int
}
func (p person) setname(name string) {
    fmt.Printf("setname中p指针变量自己的地址是%p\n",&p)
	fmt.Printf("这里设置名字用到的是值接收者方法,是原本的p的副本,所以地址和实例地址不一样,修改方法内的p的值域不会影响实例的值域\n")
    fmt.Println("------------------------------------")
    p.name = name
}
func (p *person) setage(age int) {
    fmt.Printf("setage中p指针变量自己的地址=%p\n", &p)
    fmt.Printf("setage中p指针变量指向的实例的地址=%p\n", p)
	fmt.Printf("这里设置年龄用到的是指针接收者方法,克隆了一个p指针,和外面那个p指针不是一个,所以他们地址不一样\n")
	fmt.Printf("但是他们指向的结构体实例是同一个内存地址,所以修改方法内的p的值域会影响实例的值域\n")
    fmt.Println("------------------------------------")
    p.age = age
}
func (p *person) getname() string {
    fmt.Printf("getname中p指针变量自己的地址=%p\n", &p)
    fmt.Printf("getname中p指针变量指向的实例的地址=%p\n", p)
    fmt.Println("------------------------------------")
    return p.name
}
func (p *person) getage() int {
    fmt.Printf("getage中p指针变量自己的地址=%p\n", &p)
    fmt.Printf("getage中p指针变量指向的实例的地址=%p\n", p)
    fmt.Println("------------------------------------")
    return p.age
}
func main() {

/* 这个文件是通过创建两个实例p1(指针类型的实例)和p2(值类型的实例)来说明"值接收者方法"和"指针接收者方法"
	
其中从p1开始,调用setname方法时,因为setname是值接收者方法,所以传递进去的是p1指针的一个副本,在方法内修改p.name不会影响p1实例的name字段
而调用setage方法时,因为setage是指针接收者方法,所以传递进去的是p1指针的一个克隆,但是这个克隆和p1指针指向同一个实例内存地址,所以在方法内修改p.age会影响p1实例的age字段
	
同理对于p2实例,调用setname方法时,传递进去的是p2值的一个副本,在方法内修改p.name不会影响p2实例的name字段
而调用setage方法时,传递进去的是p2值的地址的一个克隆指针,虽然指针本身地址不同但是这个克隆指针和&p2指向同一个实例内存地址,所以在方法内修改p.age会影响p2实例的age字段
	
通过这种方式可以清晰地看到值接收者方法和指针接收者方法在修改结构体字段时的不同效果 */


   // 指针类型的实例
    p1 := new(person)
    fmt.Printf("p1指针变量自己的地址=%p\n", &p1)       // 指针变量自己的地址
    fmt.Printf("p1指向的实例地址=%p\n", p1)           // 指针存储的值(实例地址)
    fmt.Printf("p1实例内容=%v\n", *p1)             // 实例内容
    p1.setname("longshuai1")
    p1.setage(21)
    fmt.Printf("这个对象的姓名是%s,年龄是%d\n",p1.getname(),p1.getage())


    // 值类型的实例
    p2 := person{}
    fmt.Printf("p2非指针实例变量地址=%p\n", &p2)            // 值变量的地址
    fmt.Printf("p2实例内容=%v\n", p2) 
    p2.setname("longshuai2")
    p2.setage(23)             // 实例内容
    fmt.Printf("这个对象的姓名是%s,年龄是%d\n",p2.getname(),p2.getage())
    
}
