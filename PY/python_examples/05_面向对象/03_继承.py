"""继承"""

class Animal:
    def __init__(self, name):
        self.name = name

    def eat(self):
        return f"{self.name} 在吃饭"

# 基本继承
class Dog(Animal):
    def bark(self):
        return f"{self.name} 在叫"

dog = Dog("旺财")
print(dog.eat())   # 旺财 在吃饭
print(dog.bark())  # 旺财 在叫

# ---------- 方法重写 ----------
class Dog2(Animal):
    def eat(self):
        return f"{self.name} 在吃骨头"

dog2 = Dog2("旺财")
print(dog2.eat())  # 旺财 在吃骨头
