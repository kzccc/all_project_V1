import os
import argparse
from pathlib import Path

def count_lines_in_file(file_path):
    """统计单个文件的行数"""
    try:
        with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
            return sum(1 for _ in f)
    except Exception as e:
        print(f"读取文件 {file_path} 时出错: {e}")
        return 0

def is_code_file(file_extension):
    """判断文件是否为代码文件"""
    # 代码文件扩展名列表（可根据需要修改）
    code_extensions = {
        '.py', '.java', '.js', '.jsx', '.ts', '.tsx', '.cpp', '.c', '.h', '.hpp',
        '.cs', '.go', '.rs', '.php', '.rb', '.swift', '.kt', '.scala', '.m',
        '.sql', '.html', '.css', '.scss', '.less', '.vue', '.sh', '.bash',
        '.yml', '.yaml', '.json', '.xml', '.toml', '.ini', '.cfg', '.gradle',
        '.kt', '.kts', '.properties'
    }
    return file_extension.lower() in code_extensions

def should_exclude_file(file_extension, exclude_extensions):
    """判断文件是否应该被排除"""
    return file_extension.lower() in exclude_extensions

def count_lines_in_directory(directory_path, exclude_extensions=None):
    """统计目录中所有代码文件的行数"""
    if exclude_extensions is None:
        exclude_extensions = {'.md', '.txt', '.log', '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.png', '.jpg', '.jpeg', '.gif', '.bmp', '.ico', '.svg'}
    
    total_lines = 0
    file_count = 0
    code_files = []
    
    directory_path = Path(directory_path)
    
    if not directory_path.exists():
        print(f"错误：路径 {directory_path} 不存在")
        return 0, 0, []
    
    print(f"正在统计目录: {directory_path.absolute()}")
    print(f"排除的文件类型: {exclude_extensions}")
    print("-" * 50)
    
    for root, dirs, files in os.walk(directory_path):
        # 排除特定目录（如.git, __pycache__等）
        dirs[:] = [d for d in dirs if d not in {'.git', '__pycache__', 'node_modules', 'venv', '.venv', 'env', 'build', 'dist', 'target', '.idea', '.vscode'}]
        
        for file in files:
            file_path = Path(root) / file
            file_extension = file_path.suffix
            
            # 检查是否应该排除
            if should_exclude_file(file_extension, exclude_extensions):
                continue
            
            # 如果指定了只统计代码文件，则检查扩展名
            if not is_code_file(file_extension):
                continue
            
            lines = count_lines_in_file(file_path)
            total_lines += lines
            file_count += 1
            code_files.append({
                'path': file_path.relative_to(directory_path),
                'lines': lines
            })
            
            if lines > 0:
                print(f"{file_path.relative_to(directory_path)}: {lines} 行")
    
    return total_lines, file_count, code_files

def main(target_path):
    # ============ 在这里设置你要统计的路径 ============
    # 方式1: 直接设置固定路径
      # Windows路径示例
    # target_path = "/home/username/projects/my_project"  # Linux/Mac路径示例
    # target_path = r"D:\work\project"  # Windows路径示例
    
    # 方式2: 设置为当前目录下的某个子目录
    # target_path = os.path.join(os.getcwd(), "src")
    
    # 方式3: 设置为当前脚本所在目录
    # target_path = os.path.dirname(os.path.abspath(__file__))
    
    # 方式4: 设置为当前工作目录
    # target_path = os.getcwd()
    
    # ============ 在这里设置要排除的文件类型 ============
    exclude_extensions = {'.md', '.txt', '.log', '.pdf', '.doc', '.docx', '.xlsx'}
    
    # ============ 执行统计 ============
    print("代码行数统计工具")
    print("=" * 50)
    
    # 检查路径是否存在
    if not os.path.exists(target_path):
        print(f"错误：路径不存在 - {target_path}")
        print("请修改脚本中的 target_path 变量为有效的路径")
        return
    
    total_lines, file_count, code_files = count_lines_in_directory(target_path, exclude_extensions)
    
    print("=" * 50)
    print(f"\n统计结果:")
    print(f"目录: {os.path.abspath(target_path)}")
    print(f"代码文件数: {file_count}")
    print(f"总代码行数: {total_lines}")
    
    # 显示前10个行数最多的文件
    if code_files:
        print(f"\n文件行数排行榜 (前10):")
        sorted_files = sorted(code_files, key=lambda x: x['lines'], reverse=True)
        for i, file_info in enumerate(sorted_files[:10], 1):
            print(f"{i:2}. {file_info['path']}: {file_info['lines']} 行")
        
        # 显示语言分布（可选）
        print(f"\n按扩展名统计:")
        ext_stats = {}
        for file_info in code_files:
            ext = Path(file_info['path']).suffix.lower()
            ext_stats[ext] = ext_stats.get(ext, 0) + file_info['lines']
        
        for ext, lines in sorted(ext_stats.items(), key=lambda x: x[1], reverse=True):
            print(f"  {ext}: {lines} 行")

if __name__ == "__main__":
    target_path = r"/workspace/czk/chenzekai_kama/KamaChat"
    main(target_path)