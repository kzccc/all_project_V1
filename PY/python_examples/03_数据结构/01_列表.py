"""列表操作"""

# ---------- 创建和访问 ----------
my_list = [1, 2, 3, "Python", True]
print(my_list[0])       # 1
print(my_list[1:4])     # [2, 3, 'Python']
print(my_list[-1])      # True
print(my_list[::2])     # [1, 3, True]

# ---------- 添加元素 ----------
my_list.append("新元素")
my_list.insert(1, "插入")
my_list.extend([4, 5])

# ---------- 删除元素 ----------
del my_list[0]
my_list.remove("Python")
popped = my_list.pop(1)

# ---------- 修改 ----------
my_list[0] = "修改"

# ---------- 常用方法 ----------
numbers = [3, 1, 4, 1, 5, 9]
print(max(numbers))         # 9
print(min(numbers))         # 1
print(len(numbers))         # 6
print(numbers.count(1))     # 2
numbers.sort(reverse=True)  # [9, 5, 4, 3, 1, 1]
numbers.reverse()           # [1, 1, 3, 4, 5, 9]

# ---------- 列表去重 ----------
list1 = ['zhangsan', 'lisi', 'wangwu', 'zhangsan']
list2 = []
for item in list1:
    if item not in list2:
        list2.append(item)
print(list2)  # ['zhangsan', 'lisi', 'wangwu']

# 使用 set 去重
list2 = list(set(list1))

# ---------- 列表推导式 ----------
squares = [x**2 for x in range(1, 6)]
print(squares)  # [1, 4, 9, 16, 25]

evens = [x for x in range(10) if x % 2 == 0]
print(evens)  # [0, 2, 4, 6, 8]
