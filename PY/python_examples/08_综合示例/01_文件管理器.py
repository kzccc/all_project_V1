"""综合示例：文件管理器"""
import os
from datetime import datetime


class FileManager:
    def __init__(self, directory):
        self.directory = directory

    def list_files(self):
        return [f for f in os.listdir(self.directory)
                if os.path.isfile(os.path.join(self.directory, f))]

    def log_files(self, log_file="file_log.txt"):
        try:
            files = self.list_files()
            with open(log_file, "a", encoding="utf-8") as f:
                f.write(f"Log at {datetime.now()}:\n")
                f.write("\n".join(map(str, files)))
                f.write("\n\n")
            print(f"文件列表已记录到 {log_file}")
        except Exception as e:
            print(f"错误：{e}")


fm = FileManager(".")
print(fm.list_files())
fm.log_files()
