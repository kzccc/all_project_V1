"""Lambda 匿名函数"""

# 基本 lambda
heshui = lambda name, duoshao: f"{name}喝了{duoshao}水"
print(heshui("张三", "1L"))  # 张三喝了1L水

# 用于排序
pairs = [(1, "one"), (3, "three"), (2, "two")]
pairs.sort(key=lambda x: x[0])
print(pairs)  # [(1, 'one'), (2, 'two'), (3, 'three')]
