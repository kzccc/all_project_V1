"""类与对象"""

class Person:
    def __init__(self, name, age):
        self.name = name
        self.age = age

    def introduce(self):
        return f"我是{self.name}，今年{self.age}岁"

    def birthday(self):
        self.age += 1
        return f"{self.name} 过生日，年龄变为 {self.age}"

p = Person("张三", 18)
print(p.introduce())  # 我是张三，今年18岁
print(p.birthday())   # 张三 过生日，年龄变为 19
