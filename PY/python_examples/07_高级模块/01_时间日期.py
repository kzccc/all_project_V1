"""时间日期模块"""
from datetime import datetime, timedelta

now = datetime.now()
print(now)
print(now.strftime("%Y-%m-%d %H:%M:%S"))  # 格式化输出

tomorrow = now + timedelta(days=1)
print(tomorrow)
