package main
//? 如果struct名称首字母是小写的，这个struct不会被导出。连同它里面的字段也不会导出，即使有首字母大写的字段名。
//? 如果struct名称首字母大写，则struct会被导出，但只会导出它内部首字母大写的字段，那些小写首字母的字段不会被导出。
//? 但并非绝对如此，如果struct嵌套了，那么即使被嵌套在内部的struct名称首字母小写，也能访问到它里面首字母大写的字段。
//? 注意这里第三点有一个问题,需要注意方法的首字母大小写问题。由于内、外stuct在同一包内，所以直接在该包内构建外部struct实例，外部struct实例是可以直接访问内部struct的所有方法的。但如果在其它包内构建外部struct实例，该实例将无法访问内部struct中首字母小写的方法。具体见""workspace/czk/chenzekai/Golang/Struct/6导出与暴露/内外结构体导出的方法可访问性问题",注意,变量字段也是如此
type animal struct{
    name string
    Speak string
}
type horse struct {
    animal
    sound string
}
//? 很多时候，Horse这个名字是不安全的，因为这表示导出Horse这个struct给其他包，也就是将Horse给暴露出去了，外界可以直接打开Horse这个"黑匣子",所以很多时候horse应该小写
//? 所以为了能够导出隐藏的struct,我们可以写一个导出的构造函数NewHorse,也就是构造器让外部可以通过这个函数来创建Horse实例
//? 如果是完全隐蔽,那就可以把传参数去掉,专门用导出函数来设置字段值
func NewHorse(name, speak, sound string) *horse {
    return &horse{
        animal: animal{
            name:  name,
            Speak: speak,
        },
        sound: sound,
    }
}
//? 虽然其它包中构建的Horse实例已经具备了name属性，但还是无法访问该实例的name属性,所以包中继续写一个可导出的方法/
func (h *horse) SetName(name string) {
    h.name = name
}
func (h *horse) GetName() string {
    return h.name
}
func main() {
    // 使用 NewHorse 创建 horse 实例
    horse := NewHorse("白马", "嘶鸣", "neigh")
    println(horse.name)   // 输出: 白马
    println(horse.Speak)  // 输出: 嘶鸣
    println(horse.sound)  // 输出: neigh
}


