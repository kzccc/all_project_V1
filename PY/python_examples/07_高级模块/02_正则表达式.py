"""正则表达式"""
import re

text = "我的邮箱是 example@domain.com"
match = re.search(r"[\w\.-]+@[\w\.-]+\.\w+", text)
if match:
    print(match.group())  # example@domain.com
