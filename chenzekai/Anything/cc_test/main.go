package main

import (
	"log"
	"student-management-system/storage"
	"student-management-system/ui"

	"fyne.io/fyne/v2/app"
)

func main() {
	// 创建 Fyne 应用
	myApp := app.New()

	// 创建存储管理器
	dataPath := "data/students.json"
	store := storage.NewStorage(dataPath)

	// 加载学生数据
	if err := store.LoadStudents(); err != nil {
		log.Printf("警告：加载学生数据失败: %v\n", err)
		log.Println("将创建新的数据文件")
	}

	// 创建并显示 GUI
	gui := ui.NewGUI(myApp, store)
	gui.Show()
}
