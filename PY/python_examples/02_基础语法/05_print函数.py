"""print 函数的各种用法"""

# 基本输出
print("Hello, Python!")

# f-string（Python 3.6+）
name = "张三"
age = 18
print(f"{name} 今年 {age} 岁")

# format 方法
print("{} 今年 {} 岁".format(name, age))

# % 运算符（旧式）
print("%s 今年 %d 岁" % (name, age))

# sep 和 end 参数
print("a", "b", "c", sep="|", end="!\n")  # a|b|c!
