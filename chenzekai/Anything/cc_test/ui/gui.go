package ui

import (
	"fmt"
	"strconv"
	"student-management-system/model"
	"student-management-system/storage"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// GUI 图形界面管理器
type GUI struct {
	app     fyne.App
	window  fyne.Window
	storage *storage.Storage
	table   *widget.Table
	data    []*model.Student
}

// NewGUI 创建新的 GUI 管理器
func NewGUI(app fyne.App, storage *storage.Storage) *GUI {
	return &GUI{
		app:     app,
		storage: storage,
		data:    []*model.Student{},
	}
}

// Show 显示主窗口
func (g *GUI) Show() {
	g.window = g.app.NewWindow("学生管理系统")
	g.window.Resize(fyne.NewSize(900, 600))

	// 加载数据
	g.refreshData()

	// 创建表格
	g.createTable()

	// 创建按钮面板
	buttonPanel := g.createButtonPanel()

	// 布局
	content := container.NewBorder(
		nil,
		nil,
		nil,
		buttonPanel,
		container.NewScroll(g.table),
	)

	g.window.SetContent(content)
	g.window.ShowAndRun()
}

// createTable 创建学生列表表格
func (g *GUI) createTable() {
	g.table = widget.NewTable(
		func() (int, int) {
			return len(g.data) + 1, 6 // +1 for header row
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)

			// 表头
			if id.Row == 0 {
				headers := []string{"学号", "姓名", "年龄", "性别", "专业", "邮箱"}
				label.SetText(headers[id.Col])
				label.TextStyle = fyne.TextStyle{Bold: true}
				return
			}

			// 数据行
			if id.Row-1 < len(g.data) {
				student := g.data[id.Row-1]
				switch id.Col {
				case 0:
					label.SetText(student.ID)
				case 1:
					label.SetText(student.Name)
				case 2:
					label.SetText(strconv.Itoa(student.Age))
				case 3:
					label.SetText(student.Gender)
				case 4:
					label.SetText(student.Major)
				case 5:
					label.SetText(student.Email)
				}
			}
		},
	)

	// 设置列宽
	g.table.SetColumnWidth(0, 100)
	g.table.SetColumnWidth(1, 100)
	g.table.SetColumnWidth(2, 60)
	g.table.SetColumnWidth(3, 60)
	g.table.SetColumnWidth(4, 150)
	g.table.SetColumnWidth(5, 200)
}

// createButtonPanel 创建按钮面板
func (g *GUI) createButtonPanel() *fyne.Container {
	addBtn := widget.NewButton("添加学生", func() {
		g.showAddDialog()
	})

	editBtn := widget.NewButton("编辑学生", func() {
		g.showEditDialog()
	})

	deleteBtn := widget.NewButton("删除学生", func() {
		g.showDeleteDialog()
	})

	refreshBtn := widget.NewButton("刷新列表", func() {
		g.refreshData()
		g.table.Refresh()
		dialog.ShowInformation("提示", "列表已刷新", g.window)
	})

	return container.NewVBox(
		addBtn,
		editBtn,
		deleteBtn,
		refreshBtn,
	)
}

// showAddDialog 显示添加学生对话框
func (g *GUI) showAddDialog() {
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("请输入学号")

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("请输入姓名")

	ageEntry := widget.NewEntry()
	ageEntry.SetPlaceHolder("请输入年龄")

	genderSelect := widget.NewSelect([]string{"男", "女"}, nil)
	genderSelect.PlaceHolder = "请选择性别"

	majorEntry := widget.NewEntry()
	majorEntry.SetPlaceHolder("请输入专业")

	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("请输入邮箱（可选）")

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "学号", Widget: idEntry},
			{Text: "姓名", Widget: nameEntry},
			{Text: "年龄", Widget: ageEntry},
			{Text: "性别", Widget: genderSelect},
			{Text: "专业", Widget: majorEntry},
			{Text: "邮箱", Widget: emailEntry},
		},
		OnSubmit: func() {
			age, err := strconv.Atoi(ageEntry.Text)
			if err != nil {
				dialog.ShowError(fmt.Errorf("年龄必须是数字"), g.window)
				return
			}

			student := &model.Student{
				ID:     idEntry.Text,
				Name:   nameEntry.Text,
				Age:    age,
				Gender: genderSelect.Selected,
				Major:  majorEntry.Text,
				Email:  emailEntry.Text,
			}

			if err := g.storage.AddStudent(student); err != nil {
				dialog.ShowError(err, g.window)
				return
			}

			g.refreshData()
			g.table.Refresh()
			dialog.ShowInformation("成功", "学生添加成功", g.window)
		},
	}

	dialogWindow := dialog.NewCustom("添加学生", "关闭", form, g.window)
	dialogWindow.Resize(fyne.NewSize(400, 400))
	dialogWindow.Show()
}

// showEditDialog 显示编辑学生对话框
func (g *GUI) showEditDialog() {
	if len(g.data) == 0 {
		dialog.ShowInformation("提示", "没有学生可以编辑", g.window)
		return
	}

	// 选择要编辑的学生
	studentIDs := make([]string, len(g.data))
	studentMap := make(map[string]*model.Student)
	for i, s := range g.data {
		studentIDs[i] = fmt.Sprintf("%s - %s", s.ID, s.Name)
		studentMap[studentIDs[i]] = s
	}

	var selectedStudent *model.Student
	selectWidget := widget.NewSelect(studentIDs, func(value string) {
		selectedStudent = studentMap[value]
	})
	selectWidget.PlaceHolder = "请选择要编辑的学生"

	selectForm := container.NewVBox(
		widget.NewLabel("选择学生："),
		selectWidget,
		widget.NewButton("确定", func() {
			if selectedStudent == nil {
				dialog.ShowError(fmt.Errorf("请选择一个学生"), g.window)
				return
			}
			g.showEditFormDialog(selectedStudent)
		}),
	)

	dialog.ShowCustom("选择学生", "取消", selectForm, g.window)
}

// showEditFormDialog 显示编辑表单对话框
func (g *GUI) showEditFormDialog(oldStudent *model.Student) {
	idEntry := widget.NewEntry()
	idEntry.SetText(oldStudent.ID)

	nameEntry := widget.NewEntry()
	nameEntry.SetText(oldStudent.Name)

	ageEntry := widget.NewEntry()
	ageEntry.SetText(strconv.Itoa(oldStudent.Age))

	genderSelect := widget.NewSelect([]string{"男", "女"}, nil)
	genderSelect.SetSelected(oldStudent.Gender)

	majorEntry := widget.NewEntry()
	majorEntry.SetText(oldStudent.Major)

	emailEntry := widget.NewEntry()
	emailEntry.SetText(oldStudent.Email)

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "学号", Widget: idEntry},
			{Text: "姓名", Widget: nameEntry},
			{Text: "年龄", Widget: ageEntry},
			{Text: "性别", Widget: genderSelect},
			{Text: "专业", Widget: majorEntry},
			{Text: "邮箱", Widget: emailEntry},
		},
		OnSubmit: func() {
			age, err := strconv.Atoi(ageEntry.Text)
			if err != nil {
				dialog.ShowError(fmt.Errorf("年龄必须是数字"), g.window)
				return
			}

			student := &model.Student{
				ID:     idEntry.Text,
				Name:   nameEntry.Text,
				Age:    age,
				Gender: genderSelect.Selected,
				Major:  majorEntry.Text,
				Email:  emailEntry.Text,
			}

			if err := g.storage.UpdateStudent(oldStudent.ID, student); err != nil {
				dialog.ShowError(err, g.window)
				return
			}

			g.refreshData()
			g.table.Refresh()
			dialog.ShowInformation("成功", "学生信息更新成功", g.window)
		},
	}

	dialogWindow := dialog.NewCustom("编辑学生", "关闭", form, g.window)
	dialogWindow.Resize(fyne.NewSize(400, 400))
	dialogWindow.Show()
}

// showDeleteDialog 显示删除学生对话框
func (g *GUI) showDeleteDialog() {
	if len(g.data) == 0 {
		dialog.ShowInformation("提示", "没有学生可以删除", g.window)
		return
	}

	// 选择要删除的学生
	studentIDs := make([]string, len(g.data))
	studentMap := make(map[string]*model.Student)
	for i, s := range g.data {
		studentIDs[i] = fmt.Sprintf("%s - %s", s.ID, s.Name)
		studentMap[studentIDs[i]] = s
	}

	var selectedStudent *model.Student
	selectWidget := widget.NewSelect(studentIDs, func(value string) {
		selectedStudent = studentMap[value]
	})
	selectWidget.PlaceHolder = "请选择要删除的学生"

	selectForm := container.NewVBox(
		widget.NewLabel("选择学生："),
		selectWidget,
		widget.NewButton("删除", func() {
			if selectedStudent == nil {
				dialog.ShowError(fmt.Errorf("请选择一个学生"), g.window)
				return
			}

			// 确认删除
			dialog.ShowConfirm(
				"确认删除",
				fmt.Sprintf("确定要删除学生 %s - %s 吗？", selectedStudent.ID, selectedStudent.Name),
				func(confirmed bool) {
					if confirmed {
						if err := g.storage.DeleteStudent(selectedStudent.ID); err != nil {
							dialog.ShowError(err, g.window)
							return
						}
						g.refreshData()
						g.table.Refresh()
						dialog.ShowInformation("成功", "学生删除成功", g.window)
					}
				},
				g.window,
			)
		}),
	)

	dialog.ShowCustom("删除学生", "取消", selectForm, g.window)
}

// refreshData 刷新数据
func (g *GUI) refreshData() {
	g.data = g.storage.GetAllStudents()
}
