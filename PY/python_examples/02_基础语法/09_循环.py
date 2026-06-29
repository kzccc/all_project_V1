"""循环语句"""

# ---------- for 循环 ----------
fruits = ["苹果", "香蕉", "橙子"]
for fruit in fruits:
    print(fruit)

# 遍历字符串
for char in "Python":
    print(char)

# range 函数
for i in range(1, 6, 2):  # start=1, stop=5, step=2
    print(i)  # 1, 3, 5

# ---------- while 循环 ----------
count = 0
while count < 5:
    print(count)
    count += 1

# ---------- 嵌套循环 ----------
for i in range(3):
    for j in range(2):
        print(f"i={i}, j={j}")

# ---------- 循环控制 ----------
for i in range(10):
    if i == 5:
        break
    if i % 2 == 0:
        continue
    print(i)  # 1, 3
