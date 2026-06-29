"""多层继承"""

class Animal:
    def __init__(self, name):
        self.name = name

    def eat(self):
        return f"{self.name} 在吃饭"

class Pet(Animal):
    def play(self):
        return f"{self.name} 在玩耍"

class SuperPet(Pet):
    def super_skill(self):
        return f"{self.name} 有超能力"

pet = SuperPet("小白")
print(pet.eat())         # 小白 在吃饭
print(pet.play())        # 小白 在玩耍
print(pet.super_skill()) # 小白 有超能力
