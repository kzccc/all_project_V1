"""文件操作"""

# ---------- 写文件 ----------
with open("example.txt", "w", encoding="utf-8") as f:
    f.write("Hello, Python!\n")
    f.write("这是第二行")

# ---------- 读文件 ----------
with open("example.txt", "r", encoding="utf-8") as f:
    content = f.read()
    print(content)

# ---------- 逐行读取 ----------
with open("example.txt", "r", encoding="utf-8") as f:
    for line in f:
        print(line.strip())

# ---------- 追加内容 ----------
with open("example.txt", "a", encoding="utf-8") as f:
    f.write("\n追加一行")
