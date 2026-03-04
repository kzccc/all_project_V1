from typing import List, Tuple, Dict, Optional
import sys

def calculate_video_progress_in_range(
    episode_durations: List[str], 
    current_episode: int, 
    current_time: str = None,
    start_episode: int = 1,
    end_episode: Optional[int] = None
) -> Dict:
    """
    计算指定区间内视频的观看进度（模式1）
    
    Args:
        episode_durations: 每集时长的列表，格式为 ["mm:ss", "mm:ss", ...]
        current_episode: 当前正在观看的集数（从1开始计数）
        current_time: 当前集已观看的时间，格式为 "mm:ss"，默认None表示看完整集
        start_episode: 区间起始集数（默认从第1集）
        end_episode: 区间结束集数（默认到最后一集）
    
    Returns:
        包含详细进度信息的字典
    """
    
    def time_to_seconds(time_str: str) -> int:
        """将 mm:ss 格式转换为秒数"""
        if not time_str:
            return 0
        minutes, seconds = map(int, time_str.split(':'))
        return minutes * 60 + seconds
    
    def seconds_to_hms(seconds: int) -> Tuple[int, int, int]:
        """将秒数转换为 (小时, 分钟, 秒)"""
        hours = seconds // 3600
        minutes = (seconds % 3600) // 60
        secs = seconds % 60
        return hours, minutes, secs
    
    def format_time(hours: int, minutes: int, seconds: int = 0) -> str:
        """格式化时间为字符串"""
        if hours > 0:
            return f"{hours}小时{minutes}分钟"
        else:
            return f"{minutes}分钟"
    
    # 设置区间边界
    total_episodes = len(episode_durations)
    if end_episode is None:
        end_episode = total_episodes
    
    # 验证参数有效性
    if not (1 <= start_episode <= end_episode <= total_episodes):
        raise ValueError(f"区间设置错误：起始集{start_episode}～结束集{end_episode}，总集数{total_episodes}")
    
    if not (start_episode <= current_episode <= end_episode):
        raise ValueError(f"当前集数{current_episode}不在区间[{start_episode}, {end_episode}]内")
    
    # 1. 计算区间总时长
    total_seconds = 0
    for i in range(start_episode - 1, end_episode):
        total_seconds += time_to_seconds(episode_durations[i])
    
    total_hours, total_minutes, _ = seconds_to_hms(total_seconds)
    
    # 2. 计算区间内已观看时长
    watched_seconds = 0
    
    # 区间内，在当前集之前的完整集
    for i in range(start_episode - 1, current_episode - 1):
        watched_seconds += time_to_seconds(episode_durations[i])
    
    # 当前集观看的时间
    if current_time is None:
        # 默认看完整集
        watched_seconds += time_to_seconds(episode_durations[current_episode - 1])
    else:
        # 只看到指定时间
        watched_seconds += time_to_seconds(current_time)
    
    watched_hours, watched_minutes, _ = seconds_to_hms(watched_seconds)
    
    # 3. 计算区间内剩余时长
    remaining_seconds = total_seconds - watched_seconds
    remaining_hours, remaining_minutes, _ = seconds_to_hms(remaining_seconds)
    
    # 4. 计算区间内百分比
    percentage = (watched_seconds / total_seconds * 100) if total_seconds > 0 else 0
    
    # 5. 整理结果
    result = {
        "range": {
            "start": start_episode,
            "end": end_episode,
            "formatted": f"第{start_episode}集～第{end_episode}集",
            "total_episodes_in_range": end_episode - start_episode + 1
        },
        "total": {
            "seconds": total_seconds,
            "formatted": format_time(total_hours, total_minutes),
            "hours": total_hours,
            "minutes": total_minutes
        },
        "watched": {
            "seconds": watched_seconds,
            "formatted": format_time(watched_hours, watched_minutes),
            "hours": watched_hours,
            "minutes": watched_minutes
        },
        "remaining": {
            "seconds": remaining_seconds,
            "formatted": format_time(remaining_hours, remaining_minutes),
            "hours": remaining_hours,
            "minutes": remaining_minutes
        },
        "percentage": round(percentage, 2),
        "current_episode": current_episode,
        "current_time": current_time if current_time else "完整",
        "total_episodes": total_episodes,
        "episode_progress": f"{current_episode}/{total_episodes}"
    }
    
    return result

def calculate_episodes_for_time(
    episode_durations: List[str],
    target_hours: int,
    target_minutes: int,
    start_episode: int = 1
) -> Dict:
    """
    计算给定时间内至少能看多少集（模式2）
    
    Args:
        episode_durations: 每集时长的列表，格式为 ["mm:ss", "mm:ss", ...]
        target_hours: 目标观看小时数
        target_minutes: 目标观看分钟数
        start_episode: 从第几集开始看（默认从第1集）
    
    Returns:
        包含计算结果的字典
    """
    
    def time_to_seconds(time_str: str) -> int:
        """将 mm:ss 格式转换为秒数"""
        if not time_str:
            return 0
        minutes, seconds = map(int, time_str.split(':'))
        return minutes * 60 + seconds
    
    def seconds_to_hms(seconds: int) -> Tuple[int, int, int]:
        """将秒数转换为 (小时, 分钟, 秒)"""
        hours = seconds // 3600
        minutes = (seconds % 3600) // 60
        secs = seconds % 60
        return hours, minutes, secs
    
    def format_time(hours: int, minutes: int, seconds: int = 0) -> str:
        """格式化时间为字符串"""
        if hours > 0:
            return f"{hours}小时{minutes}分钟"
        else:
            return f"{minutes}分钟"
    
    # 验证参数
    total_episodes = len(episode_durations)
    if not (1 <= start_episode <= total_episodes):
        raise ValueError(f"起始集数{start_episode}无效，总集数{total_episodes}")
    
    if target_hours < 0 or target_minutes < 0:
        raise ValueError("观看时间不能为负数")
    
    if target_hours == 0 and target_minutes == 0:
        raise ValueError("观看时间不能为0")
    
    # 计算目标总秒数
    target_seconds = target_hours * 3600 + target_minutes * 60
    
    # 计算累计观看的集数
    accumulated_seconds = 0
    watched_episodes = 0
    episode_info = []
    
    for i in range(start_episode - 1, total_episodes):
        episode_duration_seconds = time_to_seconds(episode_durations[i])
        
        # 如果加上这一集会超过目标时间，就停止
        if accumulated_seconds + episode_duration_seconds > target_seconds:
            # 检查是否能看一部分这一集
            remaining_time = target_seconds - accumulated_seconds
            if remaining_time > 0:
                # 可以看部分内容
                ep_hours, ep_minutes, ep_seconds = seconds_to_hms(episode_duration_seconds)
                rem_hours, rem_minutes, rem_seconds = seconds_to_hms(remaining_time)
                
                episode_info.append({
                    "episode": i + 1,
                    "duration": format_time(ep_hours, ep_minutes),
                    "watched": "部分",
                    "watched_seconds": remaining_time,
                    "watched_formatted": format_time(rem_hours, rem_minutes),
                    "percentage": round(remaining_time / episode_duration_seconds * 100, 2)
                })
                accumulated_seconds += remaining_time
                watched_episodes += 0.5  # 表示看了半集
            break
        else:
            # 可以看完这一集
            ep_hours, ep_minutes, _ = seconds_to_hms(episode_duration_seconds)
            episode_info.append({
                "episode": i + 1,
                "duration": format_time(ep_hours, ep_minutes),
                "watched": "完整",
                "watched_seconds": episode_duration_seconds,
                "watched_formatted": format_time(ep_hours, ep_minutes),
                "percentage": 100.0
            })
            accumulated_seconds += episode_duration_seconds
            watched_episodes += 1
    
    # 格式化结果
    actual_hours, actual_minutes, _ = seconds_to_hms(accumulated_seconds)
    remaining_seconds = max(0, target_seconds - accumulated_seconds)
    remaining_hours, remaining_minutes, _ = seconds_to_hms(remaining_seconds)
    
    # 计算观看效率
    efficiency = (accumulated_seconds / target_seconds * 100) if target_seconds > 0 else 0
    
    result = {
        "mode": "time_to_episodes",
        "target_time": {
            "hours": target_hours,
            "minutes": target_minutes,
            "seconds": target_seconds,
            "formatted": format_time(target_hours, target_minutes)
        },
        "actual_time": {
            "seconds": accumulated_seconds,
            "formatted": format_time(actual_hours, actual_minutes),
            "hours": actual_hours,
            "minutes": actual_minutes
        },
        "remaining_time": {
            "seconds": remaining_seconds,
            "formatted": format_time(remaining_hours, remaining_minutes),
            "hours": remaining_hours,
            "minutes": remaining_minutes
        },
        "watched_episodes": watched_episodes,
        "full_episodes": int(watched_episodes),  # 完整集数
        "partial_episode": watched_episodes % 1 > 0,  # 是否有部分观看的集
        "start_episode": start_episode,
        "end_episode": start_episode + len(episode_info) - 1,
        "episode_details": episode_info,
        "efficiency": round(efficiency, 2),
        "total_episodes": total_episodes
    }
    
    return result

def print_range_progress(result: Dict):
    """格式化打印区间进度信息（模式1）"""
    print("=" * 60)
    print("视频观看进度报告（模式1：进度查询）")
    print("=" * 60)
    print(f"视频合集：共 {result['total_episodes']} 集")
    print(f"计算区间：{result['range']['formatted']}（共{result['range']['total_episodes_in_range']}集）")
    print(f"观看进度：第 {result['current_episode']} 集，已观看：{result['current_time']}")
    print(f"区间总时长：{result['total']['formatted']}")
    print(f"区间已观看：{result['watched']['formatted']}")
    print(f"区间剩余时长：{result['remaining']['formatted']}")
    print(f"区间完成度：{result['percentage']}%")
    print("=" * 60)
    
    # 绘制进度条
    bar_length = 40
    filled_length = int(bar_length * result['percentage'] / 100)
    bar = '█' * filled_length + '░' * (bar_length - filled_length)
    print(f"[{bar}] {result['percentage']}%")
    print("=" * 60)

def print_time_to_episodes(result: Dict):
    """格式化打印时间到集数的计算结果（模式2）"""
    print("=" * 60)
    print("视频观看计算报告（模式2：时间规划）")
    print("=" * 60)
    print(f"视频合集：共 {result['total_episodes']} 集")
    print(f"起始集数：第 {result['start_episode']} 集")
    print(f"计划观看时间：{result['target_time']['formatted']}")
    print(f"实际可观看：{result['actual_time']['formatted']}")
    print(f"时间利用率：{result['efficiency']}%")
    print("-" * 60)
    
    # 计算观看的集数
    if result['partial_episode']:
        episodes_text = f"至少 {result['full_episodes']} 集完整 + 1 集部分"
    else:
        episodes_text = f"至少 {result['full_episodes']} 集完整"
    
    print(f"可以观看：{episodes_text}")
    print(f"观看范围：第 {result['start_episode']} 集 ～ 第 {result['end_episode']} 集")
    print("-" * 60)
    
    # 显示每集详情
    if result['episode_details']:
        print("详细观看计划：")
        for i, ep in enumerate(result['episode_details'], 1):
            status = "✓" if ep['watched'] == "完整" else "⏳"
            if ep['watched'] == "完整":
                print(f"  {status} 第{ep['episode']:3d}集：{ep['duration']}（完整）")
            else:
                print(f"  {status} 第{ep['episode']:3d}集：{ep['duration']}（看到 {ep['watched_formatted']}，{ep['percentage']}%）")
    
    if result['remaining_time']['seconds'] > 0:
        print(f"\n剩余时间：{result['remaining_time']['formatted']} 未使用")
    
    # 绘制进度条
    bar_length = 40
    filled_length = int(bar_length * result['efficiency'] / 100)
    bar = '█' * filled_length + '░' * (bar_length - filled_length)
    print(f"\n时间利用率：[{bar}] {result['efficiency']}%")
    print("=" * 60)

def interactive_range_progress(durations: List[str]):
    """交互式计算区间进度（模式1）"""
    print(f"视频合集共 {len(durations)} 集")
    
    while True:
        try:
            print("\n" + "-" * 40)
            print("模式1：进度查询")
            print("-" * 40)
            
            # 输入区间
            start_input = input("从第几集开始计算？(默认1): ")
            start_ep = int(start_input) if start_input.strip() else 1
            
            end_input = input(f"到第几集结束？(默认{len(durations)}): ")
            end_ep = int(end_input) if end_input.strip() else len(durations)
            
            if not (1 <= start_ep <= end_ep <= len(durations)):
                print(f"区间错误！请输入 1～{len(durations)} 的有效区间")
                continue
            
            # 输入当前进度
            current_input = input(f"当前看到第几集？({start_ep}～{end_ep}): ")
            current_ep = int(current_input)
            
            if not (start_ep <= current_ep <= end_ep):
                print(f"当前集数必须在区间 [{start_ep}, {end_ep}] 内！")
                continue
            
            time_input = input(f"第{current_ep}集看到了哪里？(格式: mm:ss，直接回车表示看完整集): ")
            
            if time_input.strip() == '':
                result = calculate_video_progress_in_range(
                    durations, 
                    current_episode=current_ep,
                    start_episode=start_ep,
                    end_episode=end_ep
                )
            else:
                # 验证时间格式
                if ':' not in time_input or len(time_input.split(':')) != 2:
                    print("时间格式错误，请使用 mm:ss 格式！")
                    continue
                result = calculate_video_progress_in_range(
                    durations,
                    current_episode=current_ep,
                    current_time=time_input,
                    start_episode=start_ep,
                    end_episode=end_ep
                )
            
            print_range_progress(result)
            
            # 询问是否继续
            print("\n选项：")
            print("  1. 重新计算区间进度")
            print("  2. 切换模式（时间规划）")
            print("  3. 退出")
            choice = input("请选择 (1/2/3): ")
            
            if choice == '2':
                interactive_time_to_episodes(durations)
                break
            elif choice == '3':
                print("感谢使用！")
                sys.exit(0)
                
        except ValueError as e:
            print(f"输入错误: {e}")
        except Exception as e:
            print(f"发生错误: {e}")

def interactive_time_to_episodes(durations: List[str]):
    """交互式计算时间到集数（模式2）"""
    print(f"视频合集共 {len(durations)} 集")
    
    while True:
        try:
            print("\n" + "-" * 40)
            print("模式2：时间规划")
            print("-" * 40)
            
            # 输入起始集
            start_input = input(f"从第几集开始看？(默认1): ")
            start_ep = int(start_input) if start_input.strip() else 1
            
            if not (1 <= start_ep <= len(durations)):
                print(f"起始集错误！请输入 1～{len(durations)} 的有效集数")
                continue
            
            # 输入观看时间
            hours_input = input("计划观看几小时？(默认0): ")
            hours = int(hours_input) if hours_input.strip() else 0
            
            minutes_input = input("计划观看几分钟？(默认30): ")
            minutes = int(minutes_input) if minutes_input.strip() else 30
            
            if hours < 0 or minutes < 0:
                print("时间不能为负数！")
                continue
            
            if hours == 0 and minutes == 0:
                print("时间不能为0！")
                continue
            
            # 计算结果
            result = calculate_episodes_for_time(
                durations,
                target_hours=hours,
                target_minutes=minutes,
                start_episode=start_ep
            )
            
            print_time_to_episodes(result)
            
            # 询问是否继续
            print("\n选项：")
            print("  1. 重新计算时间规划")
            print("  2. 切换模式（进度查询）")
            print("  3. 退出")
            choice = input("请选择 (1/2/3): ")
            
            if choice == '2':
                interactive_range_progress(durations)
                break
            elif choice == '3':
                print("感谢使用！")
                sys.exit(0)
                
        except ValueError as e:
            print(f"输入错误: {e}")
        except Exception as e:
            print(f"发生错误: {e}")

def main():
    """主函数"""
    # 使用你的数据
    episode_durations =  [
    # 第1章
    "06:21",  # 1-1 为什么操作系统是计算机基石
    "08:50",  # 1-2 如何学习才能更容易掌握操作系统
    
    # 第2章
    "26:41",  # 2-1 谈论CPU时你的大脑里应该有什么
    "17:21",  # 2-2 谈论CPU时你的大脑里应该有什么
    "25:53",  # 2-3 内存的本质是什么
    "20:11",  # 2-4 编程语言是怎么一回事
    "10:30",  # 2-5 到底什么是编译型语言以及解释
    "23:48",  # 2-6 为什么内存中有堆区和栈区
    "17:02",  # 2-7 没有操作系统程序能运行起来吗
    "22:01",  # 2-8 程序是怎样运行起来的(上)
    "19:10",  # 2-9 程序是怎样运行起来的(下)
    "14:31",  # 2-10 为什么用c语言编写操作系统
    "18:40",  # 2-11 程序和进程有什么区别(上)
    "15:22",  # 2-12 程序和进程有什么区别(下)
    
    # 第3章
    "22:46",  # 3-1 操作系统是如何实现的
    "26:26",  # 3-2 操作系统是如何启动的
    "26:19",  # 3-3 cpu权限管理的王者之争(上)
    "06:59",  # 3-4 cpu权限管理的王者之争(下)
    "17:22",  # 3-5 为什么需要系统调用什么是系统
    "20:09",  # 3-6 为什么需要系统调用什么是系统
    "15:17",  # 3-7 系统调用时CPU和操作系统中发
    "13:02",  # 3-8 系统调用时CPU和操作系统中发
    "18:04",  # 3-9 CPU是如何执行操作系统中的函
    "15:29",  # 3-10 系统调用执行完后怎么返回用
    "09:40",  # 3-11 系统调用执行完后怎么返回用
    "20:29",  # 3-12 实践篇-真实的内核调用栈
    "26:22",  # 3-13 利用strace排查各种疑难杂症
    
    # 第4章
    "19:35",  # 4-1 操作系统为什么需要进程这个概念
    "08:50",  # 4-2 操作系统是如何实现多任务的要
    "25:38",  # 4-3 进程被暂停后是怎么恢复运行的
    "14:59",  # 4-4 进程切换是如何实现的(上)
    "21:01",  # 4-5 进程切换是如何实现的(中)
    "17:31",  # 4-6 进程切换是如何实现的(下)
    "15:47",  # 4-7 进程切换和函数调用的区别
    "15:40",  # 4-8 为什么进程需要各种各样的状态
    "12:36",  # 4-9 为什么进程需要各种各样的状态
    "26:16",  # 4-10 操作系统是如何调度进程的
    "11:03",  # 4-11 操作系统是如何调度进程的
    "17:38",  # 4-13 进程是如何在各个状态之间进
    "15:22",  # 4-14 操作系统与进程的夺权之战
    "09:31",  # 4-15 操作系统与进程的夺权之战
    "15:15",  # 4-16 windows与linux下的进程创建
    "12:28",  # 4-17 windows与linux下的进程创建
    "17:26",  # 4-18 为什么linux中需要fork与exec
    "17:53",  # 4-19 为什么linux中需要fork与exec
    "17:29",  # 4-20 操作系统是如何一步步创建出
    "08:06",  # 4-21 操作系统是如何一步步创建出
    "20:01",  # 4-22 进程是如何退出的
    "16:40",  # 4-23 操作系统是如何实现进程间通
    "21:21",  # 4-24 操作系统是如何实现进程间通
    "20:24",  # 4-25 有了进程为什么还需要线程
    "18:58",  # 4-26 线程这个概念的来源、本质与
    "19:01",  # 4-27 线程是如何被创建出来的(上)
    "13:34",  # 4-28 线程是如何被创建出来的(下)
    "17:29",  # 4-29 彻底理解进程和线程的区别
    "09:18",  # 4-30 彻底理解进程和线程的区别
    "19:33",  # 4-31 利用多线程优化程序性能(上)
    "07:57",  # 4-32 利用多线程优化程序性能(下)
    "16:32",  # 4-33 有了线程为什么还有协程(上)
    "09:08",  # 4-34 有了线程为什么还有协程(下)
    "20:14",  # 4-35 实践篇多进程编程(上)
    "13:10",  # 4-36 实践篇多进程编程(下)
    "15:23",  # 4-37 多进程多线程在浏览器中的应用
    
    # 第5章
    "17:33",  # 5-1 黑客帝国与内存背后的真相
    "18:42",  # 5-2 为什么内存分配会是个问题
    "13:39",  # 5-3 程序运行需要分配哪些内存
    "13:40",  # 5-4 操作系统是如何为进程分配内存
    "13:24",  # 5-5 操作系统是如何为进程分配内存
    "26:10",  # 5-6 操作系统是如何为进程分配内存的
    "17:43",  # 5-7 虚拟地址空间的诞生
    "22:13",  # 5-8 操作系统如何为进程分配内存
    "23:40",  # 5-9 操作系统是如何为进程分配内存
    "21:07",  # 5-10 页式内存管理会带来哪些问题
    "23:14",  # 5-11 操作系统如何加速页表读取
    "13:41",  # 5-12 操作系统如何加速页表读取
    "19:31",  # 5-13 操作系统如何解决页表占用内
    "20:31",  # 5-14 页式内存管理有哪些有趣的功能
    "11:21",  # 5-15 为什么物理内存只有1G程序员
    "14:47",  # 5-16 为什么物理内存只有1G程序员
    "25:27",  # 5-17 操作系统是如何管理内存的
    "18:54",  # 5-18 实践篇-利用内存池优化程序性
    "12:22",  # 5-19 实践篇-利用内存池优化程序性
    "20:19",  # 5-20 实践篇_申请1G内存实际会消耗
    "24:21",  # 5-21 实践篇_进程在内存中是什么样
    
    # 第6章
    "15:20",  # 6-1 中断这一章要解决什么问题
    "25:49",  # 6-2 到底什么是中断
    "24:56",  # 6-3 操作系统在什么情况下开始运行
    "24:20",  # 6-4 操作系统是如何处理中断的
    "19:20",  # 6-5 程序是如何读取到网络数据的
    
    # 第7章
    "31:03",  # 7-1 并发问题的本质与根源是什么
    "24:53",  # 7-2 实践篇_实际动手感受一下并发编
    "19:35",  # 7-3 怎样从根源上解决并发问题(上)
    "13:25",  # 7-4 怎样从根源上解决并发问题(下)
    "16:51",  # 7-5 一山不容二虎_锁是如何实现的
    "17:10",  # 7-6 一山不容二虎_锁是如何实现的
    "25:13",  # 7-7 有回旋锁为什么还需要sleep锁
    "17:43",  # 7-8 闻风而动如何实现线程间的同步
    "16:23",  # 7-9 闻风而动_如何实现线程间的同步
    "16:05",  # 7-10 到底什么是信号量信号量是怎
    "25:43",  # 7-11 实践篇.用信号量解决互斥问题
    "14:38",  # 7-12 用信号量解决同步等待问题
    "25:59",  # 7-13 用信号量解决生产者消费者问
    "17:46",  # 7-14 用信号量解决生产者消费者问
    
    # 第8章
    "15:06",  # 8-1 一切央为文件是什么意思
    "29:15",  # 8-2 操作系统是如何管理设备
    "14:32",  # 8-3 磁盘是如何工作的(上)
    "14:06",  # 8-4 磁盘是如何工作的(下)
    "20:22",  # 8-5 文件这个概念是怎样实现的(上)
    "12:30",  # 8-6 文件这个概念是怎样实现的(下)
    "19:51",  # 8-7 文件系统、文件夹以及一切皆为
    "19:00",  # 8-8 文件系统、文件夹以及一切皆为
    "08:22",  # 8-9 文件系统、文件夹以及一切秋为
    "16:42",  # 8-10 操作系统是如何打开、创建以
    "10:06",  # 8-11 操作系统是如何打开、创建以
    "15:51",  # 8-12 操作系统是如何加速文件读写
    "09:51",  # 8-13 操作系统是如何加速文件读写
    
    # 第9章
    "02:00",  # 9-1 送君干里终于一别-收下这三句忠
]
    
    print("视频观看进度与规划工具")
    print("=" * 60)
    print(f"视频合集：共 {len(episode_durations)} 集")
    print("=" * 60)
    
    # 选择模式
    while True:
        print("\n请选择模式：")
        print("  1. 模式1：进度查询（根据当前观看进度计算剩余时间）")
        print("  2. 模式2：时间规划（根据可用时间计算能看多少集）")
        print("  3. 退出")
        
        mode_choice = input("请选择 (1/2/3): ")
        
        if mode_choice == '1':
            interactive_range_progress(episode_durations)
        elif mode_choice == '2':
            interactive_time_to_episodes(episode_durations)
        elif mode_choice == '3':
            print("感谢使用！")
            break
        else:
            print("无效选择，请输入 1、2 或 3")

if __name__ == "__main__":
    main()





