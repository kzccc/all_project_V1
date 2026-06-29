"""列表排序"""

# ---------- 基本排序 ----------
numbers = [3, 1, 4, 1, 5]
numbers.sort()
print(numbers)  # [1, 1, 3, 4, 5]

# ---------- 自定义排序 ----------
words = ["apple", "banana", "cherry"]
words.sort(key=len)
print(words)  # ['apple', 'banana', 'cherry']
