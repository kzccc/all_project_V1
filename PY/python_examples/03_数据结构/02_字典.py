"""字典操作"""

# ---------- 创建和访问 ----------
dic = {"name": "zhangsan", "age": 18, "hobby": "打篮球"}
print(dic["name"])                     # zhangsan
print(dic.get("age"))                  # 18
print(dic.get("不存在", "默认值"))       # 默认值

# ---------- 添加和修改 ----------
dic["name"] = "lisi"     # 修改
dic["degree"] = "本科"   # 添加

# ---------- 删除 ----------
del dic["age"]
# dic.clear()  # 清空

# ---------- 遍历 ----------
for key in dic:
    print(key, dic[key])

for key, value in dic.items():
    print(f"{key}: {value}")

for value in dic.values():
    print(value)

# ---------- 字典推导式 ----------
squares_dict = {x: x**2 for x in range(1, 5)}
print(squares_dict)  # {1: 1, 2: 4, 3: 9, 4: 16}
