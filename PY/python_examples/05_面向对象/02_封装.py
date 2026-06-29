"""封装 - 私有属性"""

class BankAccount:
    def __init__(self, owner, balance=0):
        self.owner = owner
        self.__balance = balance  # 私有属性

    def deposit(self, amount):
        if amount > 0:
            self.__balance += amount
            return f"存入 {amount}，余额：{self.__balance}"
        return "存款金额必须大于 0"

    def get_balance(self):
        return self.__balance

account = BankAccount("张三")
print(account.deposit(100))   # 存入 100，余额：100
print(account.get_balance())  # 100
