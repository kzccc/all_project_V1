"""函数定义"""

# ---------- 基本函数 ----------
def greet(name, greeting="Hello"):
    return f"{greeting}, {name}!"

print(greet("张三"))        # Hello, 张三!
print(greet("李四", "Hi"))  # Hi, 李四!

# ---------- 可变参数 ----------
def print_args(*args, **kwargs):
    print("位置参数:", args)
    print("关键字参数:", kwargs)

print_args(1, 2, 3, name="张三", age=18)
# 位置参数: (1, 2, 3)
# 关键字参数: {'name': '张三', 'age': 18}

# ---------- 文件删除函数示例 ----------
import os

def delfile(filename, *args, isfile=True):
    print("额外参数:", args)
    if isfile and os.path.exists(filename):
        print(f"删除文件: {filename}")
        # os.remove(filename)
    else:
        print("文件不存在或不是文件")

delfile("test.txt", 1, 2, 3)
