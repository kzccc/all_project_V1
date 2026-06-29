"""集合操作"""

set1 = {1, 2, 3}
set2 = {2, 3, 4}

print(set1 | set2)  # 并集: {1, 2, 3, 4}
print(set1 & set2)  # 交集: {2, 3}
print(set1 - set2)  # 差集: {1}
