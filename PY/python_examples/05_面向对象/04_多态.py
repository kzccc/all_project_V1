"""多态"""

class Animal:
    def __init__(self, name):
        self.name = name

    def eat(self):
        return f"{self.name} 在吃饭"

class Dog(Animal):
    def eat(self):
        return f"{self.name} 在吃骨头"

class Cat(Animal):
    def eat(self):
        return f"{self.name} 在吃鱼"

animals = [Dog("旺财"), Cat("咪咪")]
for animal in animals:
    print(animal.eat())
# 旺财 在吃骨头
# 咪咪 在吃鱼
