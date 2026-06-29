"""多行语句"""

a = 100
b = 200
c = 300

# 使用反斜杠换行
total = a + \
        b + \
        c
print(total)  # 600

# 更好的方式：使用括号隐式换行
total = (a +
         b +
         c)
print(total)  # 600
