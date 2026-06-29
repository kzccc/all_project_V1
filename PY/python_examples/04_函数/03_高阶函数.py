"""高阶函数 map / filter / reduce"""
from functools import reduce

numbers = [1, 2, 3, 4]

# map: 映射
squares = list(map(lambda x: x**2, numbers))
print(squares)  # [1, 4, 9, 16]

# filter: 过滤
evens = list(filter(lambda x: x % 2 == 0, numbers))
print(evens)  # [2, 4]

# reduce: 累积
sum_result = reduce(lambda x, y: x + y, numbers)
print(sum_result)  # 10
