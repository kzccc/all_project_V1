package storage

import (
	"encoding/json"
	"errors"
	"os"
	"student-management-system/model"
	"sync"
)

// Storage 存储管理器
type Storage struct {
	filePath string
	students map[string]*model.Student
	mu       sync.RWMutex
}

// NewStorage 创建新的存储管理器
func NewStorage(filePath string) *Storage {
	return &Storage{
		filePath: filePath,
		students: make(map[string]*model.Student),
	}
}

// LoadStudents 从 JSON 文件加载学生数据
func (s *Storage) LoadStudents() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// 文件不存在，创建空文件
		return s.saveToFile()
	}

	// 读取文件
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	// 如果文件为空，初始化为空 map
	if len(data) == 0 {
		return nil
	}

	// 解析 JSON
	var students []*model.Student
	if err := json.Unmarshal(data, &students); err != nil {
		return err
	}

	// 加载到内存
	s.students = make(map[string]*model.Student)
	for _, student := range students {
		s.students[student.ID] = student
	}

	return nil
}

// SaveStudents 保存学生数据到 JSON 文件
func (s *Storage) SaveStudents() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveToFile()
}

// saveToFile 内部方法：保存到文件（不加锁）
func (s *Storage) saveToFile() error {
	// 转换为切片
	students := make([]*model.Student, 0, len(s.students))
	for _, student := range s.students {
		students = append(students, student)
	}

	// 序列化为 JSON
	data, err := json.MarshalIndent(students, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(s.filePath, data, 0644)
}

// AddStudent 添加学生
func (s *Storage) AddStudent(student *model.Student) error {
	if err := student.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查学号是否已存在
	if _, exists := s.students[student.ID]; exists {
		return errors.New("学号已存在")
	}

	s.students[student.ID] = student
	return s.saveToFile()
}

// UpdateStudent 更新学生信息
func (s *Storage) UpdateStudent(id string, student *model.Student) error {
	if err := student.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查学生是否存在
	if _, exists := s.students[id]; !exists {
		return errors.New("学生不存在")
	}

	// 如果修改了学号，需要检查新学号是否已存在
	if id != student.ID {
		if _, exists := s.students[student.ID]; exists {
			return errors.New("新学号已存在")
		}
		delete(s.students, id)
	}

	s.students[student.ID] = student
	return s.saveToFile()
}

// DeleteStudent 删除学生
func (s *Storage) DeleteStudent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查学生是否存在
	if _, exists := s.students[id]; !exists {
		return errors.New("学生不存在")
	}

	delete(s.students, id)
	return s.saveToFile()
}

// GetStudent 获取单个学生
func (s *Storage) GetStudent(id string) (*model.Student, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	student, exists := s.students[id]
	if !exists {
		return nil, errors.New("学生不存在")
	}

	return student, nil
}

// GetAllStudents 获取所有学生
func (s *Storage) GetAllStudents() []*model.Student {
	s.mu.RLock()
	defer s.mu.RUnlock()

	students := make([]*model.Student, 0, len(s.students))
	for _, student := range s.students {
		students = append(students, student)
	}

	return students
}
