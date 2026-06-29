"""迭代器与生成器"""

# ---------- 迭代器 ----------
my_list = [1, 2, 3]
iterator = iter(my_list)
print(next(iterator))  # 1
print(next(iterator))  # 2

# ---------- 生成器 ----------
def fibonacci(n):
    a, b = 0, 1
    for _ in range(n):
        yield a
        a, b = b, a + b

print(list(fibonacci(5)))  # [0, 1, 1, 2, 3]
