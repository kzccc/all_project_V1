//? Go中的数组是固定长度的，数组一经声明，就无法扩大、缩减数组的长度。但Go中也有类似的动态"数组"
//? 当在Go中声明一个数组之后，会在内存中开辟一段固定长度的、连续的空间存放数组中的各个元素，这些元素的数据类型完全相同，可以是内置的简单数据类型(int、string等)，也可以是自定义的struct类型。
//? 因为Go中的数组要求数据类型固定、长度固定，所以在声明的时候需要给定长度和数据类型。
//? 必须注意，虽然我们称呼数组为int类型的数组，但数组的数据类型是两部分组成的[n]TYPE，这个整体才是数组的数据类型。所以，[5]int和[6]int是两种不同的数组类型。不同数据类型，意味着如果数组赋值给另一数组时需要数据类型转换操作，而Go默认是不会进行数据类型转换的。
//? 在Go中，当一个变量被声明之后，都会立即对其进行默认的赋0初始化。对int类型的变量会默认初始化为0，对string类型的变量会初始化为空""，对布尔类型的变量会初始化为false，对指针(引用)类型的变量会初始化为nil。

package main

import "fmt"

func main() {
    // 四种声明方式
    fmt.Println("=== 数组声明方式 ===")
    
    // 方式1: 声明固定长度的数组
    var arr1 [5]int                    // 声明一个长度为5的整数数组，元素默认为0
    fmt.Printf("方式1 - 固定长度数组: %v\n", arr1)
    
    // 方式2: 使用自动推断长度
    arr2 := [...]int{1, 2, 3, 4, 5}   // 编译器自动计算长度为5
    fmt.Printf("方式2 - 自动推断长度: %v\n", arr2)
    
    // 方式3: 部分初始化
    arr3 := [5]int{1, 2, 3}           // 只初始化前3个元素，其余为0
    fmt.Printf("方式3 - 部分初始化: %v\n", arr3)
    
    // 方式4: 声明后赋值
    var arr4 [3]int
    arr4[0] = 1
    arr4[1] = 2
    arr4[2] = 3
    fmt.Printf("方式4 - 声明后赋值: %v\n", arr4)
    
    fmt.Println("\n=== 数组访问方式 ===")
    
    arr := [4]int{10, 20, 30, 40}
    
    // 方式1: 通过索引访问
    firstElement := arr[0]
    secondElement := arr[1]
    fmt.Printf("方式1 - 通过索引访问: arr[0]=%d, arr[1]=%d\n", firstElement, secondElement)
    
    // 方式2: 修改数组元素
    arr[0] = 100
    fmt.Printf("方式2 - 修改数组元素后: %v\n", arr)
    
    // 方式3: 遍历数组（使用for循环）
    fmt.Print("方式3 - 使用for循环遍历: ")
    for i := 0; i < len(arr); i++ {
        fmt.Printf("%d ", arr[i])
    }
    fmt.Println()
    
    // 方式4: 使用range遍历
    fmt.Print("方式4 - 使用range遍历: ")
    for _, value := range arr {
        fmt.Printf("%d ", value)
    }
    fmt.Println()
}