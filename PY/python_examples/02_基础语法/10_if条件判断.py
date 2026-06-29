"""if 条件判断"""

score = 85

# 基本 if-elif-else
if score >= 90:
    print("优秀")
elif score >= 60:
    print("及格")
else:
    print("不及格")
# 输出: 及格

# 嵌套 if
if score >= 60:
    if score >= 80:
        print("良好")
    else:
        print("刚及格")
