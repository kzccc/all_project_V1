# 基础实验：固定文法的LL(1)语法分析器
# 文法 G'[E]:
# E -> T E'
# E' -> + T E' | ε
# T -> F T'
# T' -> * F T' | ε
# F -> ( E ) | i

class LL1Parser:
    def __init__(self):
        # 初始化LL(1)分析表（已经给定）
        self.parsing_table = {
            # 非终结符: {终结符: 产生式}
            'E': {
                'i': ['T', 'E\''],      # E -> T E'
                '(': ['T', 'E\'']       # E -> T E'
            },
            'E\'': {
                '+': ['+', 'T', 'E\''], # E' -> + T E'
                ')': ['ε'],             # E' -> ε
                '#': ['ε']              # E' -> ε
            },
            'T': {
                'i': ['F', 'T\''],      # T -> F T'
                '(': ['F', 'T\'']       # T -> F T'
            },
            'T\'': {
                '+': ['ε'],             # T' -> ε
                '*': ['*', 'F', 'T\''], # T' -> * F T'
                ')': ['ε'],             # T' -> ε
                '#': ['ε']              # T' -> ε
            },
            'F': {
                'i': ['i'],             # F -> i
                '(': ['(', 'E', ')']    # F -> ( E )
            }
        }
        
        # 终结符集合
        self.terminals = ['i', '+', '*', '(', ')', '#']
        
        # 非终结符集合
        self.non_terminals = ['E', 'E\'', 'T', 'T\'', 'F']
        
        # 分析过程记录
        self.analysis_steps = []
    
    def print_parsing_table(self):
        """打印LL(1)分析表"""
        print("=" * 60)
        print("LL(1)语法分析表")
        print("=" * 60)
        
        # 打印表头
        headers = ["非终结符"] + self.terminals
        print(f"{'非终结符':<10}", end="")
        for terminal in self.terminals:
            print(f"{terminal:<15}", end="")
        print()
        print("-" * 100)
        
        # 打印每一行
        for non_terminal in self.non_terminals:
            print(f"{non_terminal:<10}", end="")
            for terminal in self.terminals:
                if terminal in self.parsing_table.get(non_terminal, {}):
                    production = self.parsing_table[non_terminal][terminal]
                    # 将列表转换为字符串
                    prod_str = ' '.join(production)
                    print(f"{prod_str:<15}", end="")
                else:
                    print(f"{'':<15}", end="")
            print()
        print()
    
    def parse(self, input_string):
        """LL(1)语法分析主函数"""
        # 确保输入以#结束
        if not input_string.endswith('#'):
            input_string += '#'
        
        # 初始化栈
        stack = ['#', 'E']  # 栈底是#，栈顶是开始符号E
        
        # 初始化输入指针
        input_pointer = 0
        
        # 记录步骤
        step = 1
        
        # 清空之前的分析记录
        self.analysis_steps = []
        
        print("=" * 60)
        print("语法分析过程")
        print("=" * 60)
        print(f"{'步骤':<6}{'分析栈':<25}{'剩余输入串':<25}{'所用产生式':<20}")
        print("-" * 80)
        
        # 开始分析
        while len(stack) > 0:
            top = stack[-1]  # 栈顶符号
            current_input = input_string[input_pointer]  # 当前输入符号
            
            # 记录当前状态
            stack_str = ''.join(stack)
            remaining_input = input_string[input_pointer:]
            
            # 情况1：栈顶是终结符或#
            if top in self.terminals or top == '#':
                if top == current_input:
                    # 匹配成功
                    if top == '#':
                        # 分析成功
                        self.analysis_steps.append((step, stack_str, remaining_input, "接受"))
                        print(f"{step:<6}{stack_str:<25}{remaining_input:<25}{'接受':<20}")
                        return True
                    else:
                        # 弹出栈顶，输入指针后移
                        self.analysis_steps.append((step, stack_str, remaining_input, f"匹配 {top}"))
                        print(f"{step:<6}{stack_str:<25}{remaining_input:<25}{f'匹配 {top}':<20}")
                        stack.pop()
                        input_pointer += 1
                else:
                    # 匹配失败
                    self.analysis_steps.append((step, stack_str, remaining_input, f"错误: 期望 {top}, 但遇到 {current_input}"))
                    print(f"{step:<6}{stack_str:<25}{remaining_input:<25}{f'错误: 期望 {top}, 但遇到 {current_input}':<20}")
                    return False
            
            # 情况2：栈顶是非终结符
            elif top in self.non_terminals:
                # 查表
                if current_input in self.parsing_table[top]:
                    production = self.parsing_table[top][current_input]
                    
                    # 弹出栈顶
                    stack.pop()
                    
                    # 将产生式右部逆序压栈（除了ε）
                    if production != ['ε']:
                        # 逆序压栈
                        for symbol in reversed(production):
                            stack.append(symbol)
                    
                    # 记录产生式
                    prod_str = f"{top} -> {' '.join(production)}"
                    self.analysis_steps.append((step, stack_str, remaining_input, prod_str))
                    print(f"{step:<6}{stack_str:<25}{remaining_input:<25}{prod_str:<20}")
                else:
                    # 表中没有对应的产生式
                    self.analysis_steps.append((step, stack_str, remaining_input, f"错误: 表中无 {top}[{current_input}]"))
                    print(f"{step:<6}{stack_str:<25}{remaining_input:<25}{f'错误: 表中无 {top}[{current_input}]':<20}")
                    return False
            
            step += 1
        
        return False
    
    def analyze_string(self, input_string):
        """完整分析过程"""
        print("\n" + "="*80)
        print(f"开始分析字符串: {input_string}")
        print("="*80)
        
        # 1. 打印分析表
        self.print_parsing_table()
        
        # 2. 执行分析
        result = self.parse(input_string)
        
        # 3. 输出分析结果
        print("\n" + "="*60)
        print("分析结果")
        print("="*60)
        if result:
            print(f"✓ 字符串 '{input_string}' 是文法的合法句子")
        else:
            print(f"✗ 字符串 '{input_string}' 不是文法的合法句子")
        
        return result


# 测试函数
def test_parser():
    """测试语法分析器"""
    parser = LL1Parser()
    
    # 测试用例
    test_cases = [
        "i+i*i",      # 应该成功
        "i*i+i",      # 应该成功
        "(i+i)*i",    # 应该成功
        "i+*i",       # 应该失败
        "(i+i",       # 应该失败
        "i++i",       # 应该失败
    ]
    
    for test_str in test_cases:
        parser.analyze_string(test_str)
        print("\n\n")


# 交互式使用
def interactive_mode():
    """交互式模式"""
    parser = LL1Parser()
    
    print("LL(1)语法分析器 - 基础实验")
    print("支持的文法: E -> T E' ; E' -> + T E' | ε ; T -> F T' ; T' -> * F T' | ε ; F -> ( E ) | i")
    print("输入字符串以进行分析（输入'quit'退出）")
    print("注意: 可以使用以下终结符: i, +, *, (, )")
    print("示例: i+i*i, (i+i)*i, i")
    
    while True:
        print("\n" + "-"*60)
        user_input = input("\n请输入要分析的字符串: ").strip()
        
        if user_input.lower() == 'quit':
            print("退出程序")
            break
        
        if not user_input:
            continue
        
        parser.analyze_string(user_input)


# 主程序
if __name__ == "__main__":
    print("请选择运行模式:")
    print("1. 运行测试用例")
    print("2. 交互式模式")
    print("3. 运行单个示例")
    
    choice = input("请输入选择 (1/2/3): ").strip()
    
    if choice == '1':
        test_parser()
    elif choice == '2':
        interactive_mode()
    elif choice == '3':
        # 运行单个示例
        parser = LL1Parser()
        parser.analyze_string("i+i*i")
    else:
        print("无效选择，运行测试用例...")
        test_parser()
