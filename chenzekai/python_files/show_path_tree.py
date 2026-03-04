import os
import argparse
from pathlib import Path

class TreeGenerator:
    """生成美观的文件夹树形结构"""
    
    # 不同的树形样式
    STYLES = {
        'classic': {'branch': '├── ', 'last': '└── ', 'vertical': '│   ', 'space': '    '},
        'rounded': {'branch': '├── ', 'last': '╰── ', 'vertical': '│   ', 'space': '    '},
        'simple': {'branch': '|-- ', 'last': '`-- ', 'vertical': '|   ', 'space': '    '},
        'double': {'branch': '╠══ ', 'last': '╚══ ', 'vertical': '║   ', 'space': '    '},
        'arrow': {'branch': '▶── ', 'last': '▶── ', 'vertical': '│   ', 'space': '    '},
    }
    
    # 文件类型图标
    ICONS = {
        'folder': '📁 ',
        'file': '📄 ',
        'image': '🖼️ ',
        'code': '📝 ',
        'document': '📄 ',
        'audio': '🎵 ',
        'video': '🎬 ',
        'archive': '🗜️ ',
        'executable': '⚙️ ',
    }
    
    def __init__(self, style='classic', show_icons=True, color_output=True):
        self.style = style if style in self.STYLES else 'classic'
        self.show_icons = show_icons
        self.color_output = color_output
        self.symbols = self.STYLES[self.style]
        
        # 文件扩展名到图标/类别的映射
        self.file_types = {
            'images': {'.png', '.jpg', '.jpeg', '.gif', '.bmp', '.svg', '.webp', '.ico'},
            'code': {'.py', '.js', '.java', '.cpp', '.c', '.h', '.html', '.css', '.php', '.rb', '.go', '.rs', '.ts'},
            'documents': {'.txt', '.md', '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx'},
            'audio': {'.mp3', '.wav', '.flac', '.aac', '.ogg', '.m4a'},
            'video': {'.mp4', '.avi', '.mov', '.mkv', '.flv', '.wmv'},
            'archives': {'.zip', '.rar', '.7z', '.tar', '.gz', '.bz2'},
            'executables': {'.exe', '.sh', '.bat', '.cmd', '.app'},
        }
    
    def get_file_icon(self, file_path):
        """根据文件扩展名获取对应的图标"""
        if not self.show_icons:
            return ""
            
        ext = os.path.splitext(file_path)[1].lower()
        
        if file_path.endswith(os.path.sep) or os.path.isdir(file_path):
            return self.ICONS['folder']
        elif ext in self.file_types['images']:
            return self.ICONS['image']
        elif ext in self.file_types['code']:
            return self.ICONS['code']
        elif ext in self.file_types['documents']:
            return self.ICONS['document']
        elif ext in self.file_types['audio']:
            return self.ICONS['audio']
        elif ext in self.file_types['video']:
            return self.ICONS['video']
        elif ext in self.file_types['archives']:
            return self.ICONS['archive']
        elif ext in self.file_types['executables']:
            return self.ICONS['executable']
        else:
            return self.ICONS['file']
    
    def colorize(self, text, item_type='folder'):
        """为输出添加颜色（如果支持）"""
        if not self.color_output:
            return text
            
        colors = {
            'folder': '\033[1;34m',  # 蓝色
            'file': '\033[0m',       # 默认
            'code': '\033[1;32m',    # 绿色
            'image': '\033[1;35m',   # 紫色
            'reset': '\033[0m',
        }
        
        color = colors.get(item_type, colors['file'])
        return f"{color}{text}{colors['reset']}"
    
    def get_item_type(self, name, is_dir):
        """获取项目类型用于着色"""
        if is_dir:
            return 'folder'
        
        ext = os.path.splitext(name)[1].lower()
        if ext in self.file_types['code']:
            return 'code'
        elif ext in self.file_types['images']:
            return 'image'
        else:
            return 'file'
    
    def generate_tree(self, root_dir, prefix="", is_last=True, max_depth=None, current_depth=0, 
                     show_hidden=False, dirs_only=False, sort_by='name'):
        """
        递归生成目录树
        
        Args:
            root_dir: 根目录路径
            prefix: 前缀字符串，用于显示层级关系
            is_last: 当前项是否是父目录的最后一项
            max_depth: 最大遍历深度
            current_depth: 当前深度
            show_hidden: 是否显示隐藏文件
            dirs_only: 是否只显示目录
            sort_by: 排序方式 ('name', 'type', 'size')
        """
        if max_depth is not None and current_depth > max_depth:
            return []
        
        try:
            items = list(os.scandir(root_dir))
        except (PermissionError, OSError):
            return [f"{prefix}{self.symbols['last'] if is_last else self.symbols['branch']}[权限拒绝]"]
        
        # 过滤隐藏文件
        if not show_hidden:
            items = [item for item in items if not item.name.startswith('.')]
        
        # 过滤目录/文件
        if dirs_only:
            items = [item for item in items if item.is_dir()]
        
        # 排序
        if sort_by == 'type':
            items.sort(key=lambda x: (not x.is_dir(), x.name.lower()))
        elif sort_by == 'size':
            # 注意：获取文件大小可能较慢
            def get_size(item):
                try:
                    return item.stat().st_size if item.is_file() else 0
                except:
                    return 0
            items.sort(key=lambda x: (not x.is_dir(), get_size(x), x.name.lower()))
        else:  # 'name'
            items.sort(key=lambda x: (not x.is_dir(), x.name.lower()))
        
        lines = []
        dir_count = 0
        file_count = 0
        
        for i, item in enumerate(items):
            is_item_last = (i == len(items) - 1)
            item_prefix = self.symbols['last'] if is_item_last else self.symbols['branch']
            
            # 构建当前项的前缀
            connector = prefix + item_prefix
            icon = self.get_file_icon(item.name)
            item_type = self.get_item_type(item.name, item.is_dir())
            
            # 显示名称
            display_name = self.colorize(icon + item.name, item_type)
            lines.append(f"{connector}{display_name}")
            
            if item.is_dir():
                dir_count += 1
                # 计算下一级的前缀
                next_prefix = prefix + (self.symbols['space'] if is_item_last else self.symbols['vertical'])
                # 递归处理子目录
                sub_lines, sub_dirs, sub_files = self.generate_tree(
                    item.path, next_prefix, is_item_last, max_depth, 
                    current_depth + 1, show_hidden, dirs_only, sort_by
                )
                lines.extend(sub_lines)
                dir_count += sub_dirs
                file_count += sub_files
            else:
                file_count += 1
        
        return lines, dir_count, file_count
    
    def print_summary(self, root_dir, dir_count, file_count, execution_time):
        """打印统计信息"""
        print("\n" + "=" * 60)
        print(f"📊 统计信息:")
        print(f"   根目录: {root_dir}")
        print(f"   文件夹: {dir_count} 个")
        print(f"   文件: {file_count} 个")
        print(f"   总计: {dir_count + file_count} 个项目")
        print(f"   生成时间: {execution_time:.2f} 秒")
        print("=" * 60)

def main():
    parser = argparse.ArgumentParser(description='生成美观的文件夹树形结构')
    parser.add_argument('path', nargs='?', default='.', help='要显示的目录路径（默认为当前目录）')
    parser.add_argument('-d', '--depth', type=int, default=None, help='最大深度')
    parser.add_argument('-H', '--hidden', action='store_true', help='显示隐藏文件')
    parser.add_argument('-D', '--dirs-only', action='store_true', help='仅显示目录')
    parser.add_argument('-s', '--style', choices=['classic', 'rounded', 'simple', 'double', 'arrow'], 
                       default='classic', help='树形样式')
    parser.add_argument('--no-icons', action='store_true', help='不显示图标')
    parser.add_argument('--no-color', action='store_true', help='不显示颜色')
    parser.add_argument('--sort', choices=['name', 'type', 'size'], default='name', help='排序方式')
    
    args = parser.parse_args()
    
    # 验证路径
    root_path = Path(args.path).resolve()
    if not root_path.exists():
        print(f"❌ 错误: 路径 '{args.path}' 不存在")
        return
    if not root_path.is_dir():
        print(f"❌ 错误: '{args.path}' 不是一个目录")
        return
    
    import time
    start_time = time.time()
    
    # 创建树生成器
    tree = TreeGenerator(
        style=args.style,
        show_icons=not args.no_icons,
        color_output=not args.no_color
    )
    
    # 打印标题
    print(f"\n🌳 目录树: {root_path}")
    print("─" * 60)
    
    # 生成并打印树形结构
    lines, dir_count, file_count = tree.generate_tree(
        str(root_path),
        max_depth=args.depth,
        show_hidden=args.hidden,
        dirs_only=args.dirs_only,
        sort_by=args.sort
    )
    
    for line in lines:
        print(line)
    
    # 打印统计信息
    execution_time = time.time() - start_time
    tree.print_summary(str(root_path), dir_count, file_count, execution_time)

if __name__ == "__main__":
    main()