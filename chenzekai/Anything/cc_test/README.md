# 学生管理系统

一个使用 Go 语言和 Fyne 框架开发的桌面学生管理系统。

## 功能特性

- ✅ 学生信息的增删改查（CRUD）
- ✅ 图形化界面，操作简单直观
- ✅ 数据持久化存储（JSON 文件）
- ✅ 数据验证（学号唯一性、年龄范围、邮箱格式等）
- ✅ 跨平台支持（Windows、Linux、macOS）

## 项目结构

```
cc_test/
├── main.go              # 程序入口
├── go.mod               # Go 模块文件
├── go.sum               # 依赖锁定文件
├── model/
│   └── student.go       # 学生数据模型
├── storage/
│   └── json_storage.go  # JSON 文件存储实现
├── ui/
│   └── gui.go           # GUI 界面实现
└── data/
    └── students.json    # 学生数据文件（运行时自动生成）
```

## 系统要求

### Linux
需要安装以下开发库：
```bash
# Ubuntu/Debian
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev

# Fedora/RHEL
sudo dnf install -y gcc mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel

# Arch Linux
sudo pacman -S gcc mesa libxcursor libxrandr libxinerama libxi
```

### Windows
- 需要安装 GCC（推荐使用 TDM-GCC 或 MinGW-w64）
- 或者使用 `go build -ldflags -H=windowsgui` 编译

### macOS
- 需要安装 Xcode Command Line Tools
```bash
xcode-select --install
```

## 安装和运行

### 1. 克隆或下载项目

```bash
cd /workspace/czk/chenzekai/Anything/cc_test
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 运行程序

```bash
go run main.go
```

### 4. 编译可执行文件

```bash
# Linux/macOS
go build -o student-management-system

# Windows
go build -o student-management-system.exe
```

## 使用说明

### 主界面

程序启动后会显示主窗口，包含：
- **左侧**：学生列表表格（显示学号、姓名、年龄、性别、专业、邮箱）
- **右侧**：操作按钮面板

### 添加学生

1. 点击"添加学生"按钮
2. 在弹出的对话框中填写学生信息：
   - 学号（必填，唯一）
   - 姓名（必填）
   - 年龄（必填，1-150）
   - 性别（必选：男/女）
   - 专业（必填）
   - 邮箱（可选，需符合邮箱格式）
3. 点击"Submit"保存

### 编辑学生

1. 点击"编辑学生"按钮
2. 在下拉列表中选择要编辑的学生
3. 点击"确定"
4. 修改学生信息
5. 点击"Submit"保存

### 删除学生

1. 点击"删除学生"按钮
2. 在下拉列表中选择要删除的学生
3. 点击"删除"
4. 在确认对话框中点击"Yes"确认删除

### 刷新列表

点击"刷新列表"按钮可以重新加载数据并刷新显示。

## 数据存储

学生数据存储在 `data/students.json` 文件中，格式如下：

```json
[
  {
    "id": "2021001",
    "name": "张三",
    "age": 20,
    "gender": "男",
    "major": "计算机科学",
    "email": "zhangsan@example.com"
  },
  {
    "id": "2021002",
    "name": "李四",
    "age": 21,
    "gender": "女",
    "major": "软件工程",
    "email": "lisi@example.com"
  }
]
```

## 技术栈

- **语言**：Go 1.21+
- **GUI 框架**：Fyne v2.7.2
- **数据存储**：JSON 文件
- **并发安全**：sync.RWMutex

## 特性说明

### 数据验证

- 学号唯一性检查
- 年龄范围验证（1-150）
- 性别限制（男/女）
- 邮箱格式验证（正则表达式）

### 并发安全

存储层使用读写锁（`sync.RWMutex`）保证并发安全。

### 错误处理

所有操作都有完善的错误处理和用户提示。

## 常见问题

### Q: 编译时提示找不到 OpenGL 库？
A: 需要安装系统的 OpenGL 开发库，参考"系统要求"部分。

### Q: 数据文件在哪里？
A: 数据文件位于 `data/students.json`，首次运行时会自动创建。

### Q: 如何备份数据？
A: 直接复制 `data/students.json` 文件即可。

### Q: 可以在没有图形界面的服务器上运行吗？
A: 不可以，这是一个桌面 GUI 应用，需要图形界面支持。

## 未来改进

- [ ] 添加搜索和筛选功能
- [ ] 支持导入/导出 Excel
- [ ] 添加成绩管理模块
- [ ] 添加课程管理模块
- [ ] 支持数据库存储（SQLite/MySQL）
- [ ] 添加用户认证系统

## 许可证

MIT License

## 作者

学生管理系统 v1.0
