"""异常处理"""

# ---------- 特定异常捕获 ----------
try:
    result = 10 / 0
except ZeroDivisionError:
    print("除数不能为 0")
except ValueError:
    print("值错误")
else:
    print("计算成功")
finally:
    print("无论如何都会执行")
# 除数不能为 0
# 无论如何都会执行

# ---------- 捕获所有异常 ----------
try:
    result = 10 / 0
except Exception as e:
    print(f"发生错误：{e}")
