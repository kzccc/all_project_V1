# 编译原理实验：正则表达式到最简DFA转换
# 探索实验(一)：依据表示标识符的正规式给出最简DFA的状态转换图

class State:
    """状态类"""
    def __init__(self, id):
        self.id = id
        self.transitions = {}  # 字符 -> 状态集合
        self.epsilon = set()   # ε转移
        self.is_final = False

class NFA:
    """非确定有限自动机"""
    def __init__(self, start, end):
        self.start = start
        self.end = end

class DFAState:
    """DFA状态类"""
    def __init__(self, id, nfa_states):
        self.id = id
        self.nfa_states = frozenset(nfa_states)  # 不可变集合
        self.transitions = {}
        self.is_final = False

class RegexToDFA:
    """正则表达式到最简DFA转换器"""
    
    def __init__(self):
        self.state_id = 0
        self.dfa_state_id = 0
        
    def new_state(self):
        """创建新状态"""
        state = State(self.state_id)
        self.state_id += 1
        return state
    
    def basic_nfa(self, char):
        """构建基本字符的NFA"""
        start = self.new_state()
        end = self.new_state()
        end.is_final = True
        
        start.transitions[char] = {end}
        return NFA(start, end)
    
    def union_nfa(self, nfa1, nfa2):
        """构建选择操作的NFA"""
        start = self.new_state()
        end = self.new_state()
        end.is_final = True
        
        nfa1.end.is_final = False
        nfa2.end.is_final = False
        
        start.epsilon.add(nfa1.start)
        start.epsilon.add(nfa2.start)
        nfa1.end.epsilon.add(end)
        nfa2.end.epsilon.add(end)
        
        return NFA(start, end)
    
    def concat_nfa(self, nfa1, nfa2):
        """构建连接操作的NFA"""
        nfa1.end.epsilon.add(nfa2.start)
        nfa1.end.is_final = False
        return NFA(nfa1.start, nfa2.end)
    
    def closure_nfa(self, nfa):
        """构建闭包操作的NFA"""
        start = self.new_state()
        end = self.new_state()
        end.is_final = True
        
        nfa.end.is_final = False
        
        start.epsilon.add(nfa.start)
        start.epsilon.add(end)
        nfa.end.epsilon.add(end)
        nfa.end.epsilon.add(nfa.start)
        
        return NFA(start, end)
    
    def to_postfix(self, regex):
        """将中缀正则表达式转换为后缀表达式"""
        precedence = {'*': 4, '.': 3, '|': 2}
        output = []
        stack = []
        
        # 添加显式的连接运算符
        regex_with_concat = self.add_concat(regex)
        
        for char in regex_with_concat:
            if char.isalnum() or char == '_':
                output.append(char)
            elif char == '(':
                stack.append(char)
            elif char == ')':
                while stack and stack[-1] != '(':
                    output.append(stack.pop())
                stack.pop()  # 弹出 '('
            else:
                while (stack and stack[-1] != '(' and
                       precedence.get(stack[-1], 0) >= precedence.get(char, 0)):
                    output.append(stack.pop())
                stack.append(char)
        
        while stack:
            output.append(stack.pop())
        
        return ''.join(output)
    
    def add_concat(self, regex):
        """添加连接运算符 ."""
        result = []
        ops = set('|*.()')
        
        for i, c in enumerate(regex):
            result.append(c)
            if i < len(regex) - 1:
                next_c = regex[i + 1]
                if (c not in ops or c in ')*') and (next_c not in ops or next_c == '('):
                    result.append('.')
        
        return ''.join(result)
    
    def build_nfa(self, regex):
        """从正则表达式构建NFA"""
        postfix = self.to_postfix(regex)
        print(f"后缀表达式: {postfix}")
        
        stack = []
        
        for char in postfix:
            if char.isalnum() or char == '_':
                stack.append(self.basic_nfa(char))
            elif char == '|':
                nfa2 = stack.pop()
                nfa1 = stack.pop()
                stack.append(self.union_nfa(nfa1, nfa2))
            elif char == '.':
                nfa2 = stack.pop()
                nfa1 = stack.pop()
                stack.append(self.concat_nfa(nfa1, nfa2))
            elif char == '*':
                nfa1 = stack.pop()
                stack.append(self.closure_nfa(nfa1))
        
        return stack.pop() if stack else None
    
    def get_alphabet_from_regex(self, regex):
        """从正则表达式提取字母表"""
        alphabet = set()
        for char in regex:
            if char.isalnum() or char == '_':
                alphabet.add(char)
        return alphabet
    
    def epsilon_closure(self, states):
        """计算ε闭包"""
        closure = set(states)
        stack = list(states)
        
        while stack:
            state = stack.pop()
            for eps_state in state.epsilon:
                if eps_state not in closure:
                    closure.add(eps_state)
                    stack.append(eps_state)
        
        return closure
    
    def move(self, states, char):
        """计算move集合"""
        result = set()
        for state in states:
            if char in state.transitions:
                result.update(state.transitions[char])
        return result
    
    def nfa_to_dfa(self, nfa, alphabet):
        """NFA转换为DFA（子集构造法）"""
        # 初始状态
        start_set = self.epsilon_closure({nfa.start})
        
        dfa_states = {}
        unmarked = []
        
        # 创建初始DFA状态
        dfa_start = DFAState(self.dfa_state_id, start_set)
        self.dfa_state_id += 1
        dfa_start.is_final = any(s.is_final for s in start_set)
        dfa_states[dfa_start.nfa_states] = dfa_start
        unmarked.append(dfa_start)
        
        # 处理所有未标记状态
        while unmarked:
            dfa_state = unmarked.pop()
            
            for char in alphabet:
                move_set = self.move(dfa_state.nfa_states, char)
                if not move_set:
                    continue
                
                closure_set = self.epsilon_closure(move_set)
                closure_frozenset = frozenset(closure_set)
                
                if closure_frozenset not in dfa_states:
                    new_dfa_state = DFAState(self.dfa_state_id, closure_set)
                    self.dfa_state_id += 1
                    new_dfa_state.is_final = any(s.is_final for s in closure_set)
                    dfa_states[closure_frozenset] = new_dfa_state
                    unmarked.append(new_dfa_state)
                
                dfa_state.transitions[char] = dfa_states[closure_frozenset].id
        
        return list(dfa_states.values()), dfa_start.id
    
    def minimize_dfa(self, dfa_states, start_id, alphabet):
        """最小化DFA"""
        # 划分：终态和非终态
        partitions = []
        final_states = {s.id for s in dfa_states if s.is_final}
        non_final_states = {s.id for s in dfa_states if not s.is_final}
        
        if final_states:
            partitions.append(final_states)
        if non_final_states:
            partitions.append(non_final_states)
        
        # 状态id到状态对象的映射
        state_map = {state.id: state for state in dfa_states}
        
        # 不断划分直到稳定
        while True:
            new_partitions = []
            
            for group in partitions:
                if len(group) <= 1:
                    new_partitions.append(group)
                    continue
                
                # 根据转移将状态分组
                subgroups = {}
                for state_id in group:
                    state = state_map[state_id]
                    key = []
                    for char in alphabet:
                        trans_id = state.transitions.get(char, -1)
                        # 找到转移状态所在的组
                        group_idx = -1
                        for i, g in enumerate(partitions):
                            if trans_id in g:
                                group_idx = i
                                break
                        key.append(str(group_idx))
                    key = tuple(key)
                    
                    if key not in subgroups:
                        subgroups[key] = set()
                    subgroups[key].add(state_id)
                
                new_partitions.extend(subgroups.values())
            
            if len(new_partitions) == len(partitions):
                break
            partitions = new_partitions
        
        # 构建最小DFA
        min_states = []
        id_map = {}
        
        for i, group in enumerate(partitions):
            # 选择组中第一个状态作为代表
            rep_id = next(iter(group))
            rep_state = state_map[rep_id]
            
            min_state = DFAState(i, rep_state.nfa_states)
            min_state.is_final = rep_state.is_final
            
            # 映射旧id到新id
            for old_id in group:
                id_map[old_id] = i
            
            min_states.append(min_state)
        
        # 设置转移
        for min_state in min_states:
            # 找到对应的原状态
            for state in dfa_states:
                if id_map[state.id] == min_state.id:
                    rep_state = state
                    break
            
            for char in alphabet:
                if char in rep_state.transitions:
                    old_target = rep_state.transitions[char]
                    min_state.transitions[char] = id_map[old_target]
        
        # 新的起始状态
        min_start = id_map[start_id]
        
        return min_states, min_start
    
    def run(self, regex, test_strings):
        """运行整个转换流程"""
        print(f"\n{'='*50}")
        print("正则表达式到最简DFA转换")
        print(f"{'='*50}")
        print(f"输入的正则表达式: {regex}")
        
        # 0. 提取字母表
        alphabet = self.get_alphabet_from_regex(regex)
        print(f"字母表: {sorted(alphabet)}")
        
        # 1. 构建NFA
        print("\n[1] 构建NFA...")
        nfa = self.build_nfa(regex)
        if not nfa:
            print("错误：无法构建NFA")
            return
        
        # 2. NFA转DFA
        print("\n[2] NFA转换为DFA...")
        dfa_states, start_id = self.nfa_to_dfa(nfa, alphabet)
        
        print(f"DFA状态数: {len(dfa_states)}")
        
        # 3. 最小化DFA
        print("\n[3] DFA最小化...")
        min_states, min_start = self.minimize_dfa(dfa_states, start_id, alphabet)
        
        print(f"最小化后状态数: {len(min_states)}")
        
        # 4. 输出转换矩阵
        print("\n[4] 最简DFA状态转换矩阵:")
        print("状态\\符号", end="")
        sorted_alphabet = sorted(alphabet)
        for char in sorted_alphabet:
            print(f" | {char:2s} ", end="")
        print()
        print("-" * (len(sorted_alphabet) * 6 + 10))
        
        for state in min_states:
            prefix = ""
            if state.id == min_start:
                prefix += "→"
            if state.is_final:
                prefix += "*"
            print(f"{prefix}q{state.id:<2d} ", end="")
            
            for char in sorted_alphabet:
                if char in state.transitions:
                    print(f"| q{state.transitions[char]} ", end="")
                else:
                    print(f"|  - ", end="")
            print()
        
        # 5. 测试字符串
        print("\n[5] 测试字符串识别:")
        for test_str in test_strings:
            current = min_start
            accepted = True
            
            for char in test_str:
                if current >= len(min_states):
                    accepted = False
                    break
                state = min_states[current]
                if char in state.transitions:
                    current = state.transitions[char]
                else:
                    accepted = False
                    break
            
            if accepted and current < len(min_states) and min_states[current].is_final:
                print(f"  ✓ '{test_str}' 被接受")
            else:
                print(f"  ✗ '{test_str}' 被拒绝")
        
        # 6. 输出状态转换图
        print("\n[6] 状态转换图描述:")
        print("digraph G {")
        print("  rankdir=LR;")
        print("  node [shape = circle];")
        
        for state in min_states:
            if state.is_final:
                print(f"  q{state.id} [shape = doublecircle];")
            else:
                print(f"  q{state.id} [shape = circle];")
            
            if state.id == min_start:
                print(f"  start [shape = point];")
                print(f"  start -> q{state.id};")
            
            for char in sorted_alphabet:
                if char in state.transitions:
                    print(f"  q{state.id} -> q{state.transitions[char]} [label = \"{char}\"];")
        print("}")

# 主程序
if __name__ == "__main__":
    print("探索实验(一)：正则表达式到最简DFA转换\n")
    
    # 测试用例1：标识符的正则表达式
    # 使用a代表字母，d代表数字
    regex1 = "a(a|d)*"
    
    test_strings1 = [
        "a",      # 有效：单个字母
        "ad",     # 有效：字母+数字
        "aa",     # 有效：字母+字母
        "add",    # 有效：字母+数字+数字
        "ada",    # 有效：字母+数字+字母
        "1a",     # 无效：数字开头
        "123",    # 无效：全是数字
        "",       # 无效：空字符串
    ]
    
    converter1 = RegexToDFA()
    print("="*60)
    print("测试用例1：标识符正则表达式 a(a|d)*")
    print("说明：a 代表字母，d 代表数字")
    print("="*60)
    converter1.run(regex1, test_strings1)
    
    # 测试用例2：更复杂的正则表达式
    print("\n" + "="*60)
    print("测试用例2：正则表达式 (a|b)*abb")
    print("="*60)
    
    regex2 = "(a|b)*abb"
    test_strings2 = ["abb", "aabb", "babb", "ababb", "aababb", "abc", "ab", "bab"]
    
    converter2 = RegexToDFA()
    converter2.run(regex2, test_strings2)