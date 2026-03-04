package main

import "fmt"

func main() {
    // 四种二维数组声明方式
    fmt.Println("=== 二维数组声明方式 ===")
    
    // 方式1: 直接声明固定大小的二维数组并初始化
    var matrix1 [3][4]int
    fmt.Printf("方式1 - 声明未初始化的二维数组: %v\n", matrix1)
    
    // 方式2: 声明并逐个初始化
    matrix2 := [3][4]int{
        {1, 2, 3, 4},
        {5, 6, 7, 8},
        {9, 10, 11, 12},
    }
    fmt.Printf("方式2 - 完全初始化的二维数组: %v\n", matrix2)
    
    // 方式3: 部分初始化
    matrix3 := [3][4]int{
        {1, 2},         // 只初始化前两个元素，其余为0
        {5, 6, 7, 8},   // 完全初始化这一行
        {9},            // 只初始化第一个元素，其余为0
    }
    fmt.Printf("方式3 - 部分初始化的二维数组: %v\n", matrix3)
    
    // 方式4: 使用省略号让编译器自动计算第一维长度
    matrix4 := [...][3]int{
        {1, 2, 3},
        {4, 5, 6},
        {7, 8, 9},
        {10, 11, 12},
    }
    fmt.Printf("方式4 - 自动计算第一维长度的二维数组: %v\n", matrix4)
    
    fmt.Println("\n=== 二维数组访问方式 ===")
    
    // 初始化一个二维数组用于演示访问
    matrix := [3][4]int{
        {1, 2, 3, 4},
        {5, 6, 7, 8},
        {9, 10, 11, 12},
    }
    
    // 方式1: 通过索引访问特定元素
    element := matrix[1][2]  // 访问第2行第3列的元素（值为7）
    fmt.Printf("方式1 - 通过索引访问: matrix[1][2] = %d\n", element)
    
    // 方式2: 修改二维数组元素
    matrix[0][0] = 100
    fmt.Printf("方式2 - 修改元素后第一行: %v\n", matrix[0])
    
    // 方式3: 遍历二维数组（使用嵌套for循环）
    fmt.Println("方式3 - 使用嵌套for循环遍历:")
    for i := 0; i < len(matrix); i++ {
        for j := 0; j < len(matrix[i]); j++ {
            fmt.Printf("%d ", matrix[i][j])
        }
        fmt.Println()
    }
    
    // 方式4: 使用range遍历二维数组
    fmt.Println("方式4 - 使用range遍历二维数组:")
    for rowIndex, row := range matrix {
        for colIndex, value := range row {
            fmt.Printf("matrix[%d][%d] = %d\t", rowIndex, colIndex, value)
        }
        fmt.Println()
    }
}
