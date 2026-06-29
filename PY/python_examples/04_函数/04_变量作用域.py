"""变量作用域"""

# ---------- 全局和局部变量 ----------
a = 2  # 全局变量

def test():
    b = 3  # 局部变量
    print(a, b)

test()  # 2 3

# ---------- global 关键字 ----------
a = 2

def modify_global():
    global a
    a = 10

modify_global()
print(a)  # 10

# ---------- nonlocal 关键字 ----------
def outer():
    x = 10

    def inner():
        nonlocal x
        x += 5

    inner()
    print(x)  # 15

outer()
