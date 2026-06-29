# RK356x 视觉识别实训项目

本项目是一个运行在 Rockchip RK356x / RK3568 开发板上的嵌入式 Linux 视觉识别实训程序。项目不需要重新烧写系统，通过 SSH / SCP 部署到板子后即可运行，并支持开机自启动。

程序实现了 LCD 图形界面、触摸交互、摄像头实时预览、YOLOv5 目标检测、带框抓拍、图库管理、Wi-Fi 管理和识别到人自动抓拍等功能。

## 功能概览

- LCD 屏幕显示主界面、实时视频、检测框、图库和 Wi-Fi 页面
- 触摸屏按钮交互，支持图库左右滑动切换
- USB 摄像头实时采集和预览
- RKNN Runtime 加载 YOLOv5 模型进行目标检测
- 检测结果显示可读类别名，例如 `person`
- 手动抓拍保存带检测框的图片
- 图库查看、上一张、下一张、删除图片
- 自动抓拍开关，识别到 `person` 后自动保存图片
- Wi-Fi 扫描、密码输入、连接和本机 IP 显示
- 通过 init 脚本实现开机自启动

## 目录结构

```text
rk356x_demo
├── Makefile
├── README.md
├── src/
│   └── main.cpp
├── model/
│   ├── yolov5s_rk3568.rknn
│   ├── coco_80_labels_list.txt
│   └── anchors_yolov5.txt
├── lib/
│   └── librknnrt.so
├── third_party/
│   └── rknn/
│       ├── rknn_api.h
│       └── librknnrt.so
├── scripts/
│   ├── build_in_container.sh
│   ├── deploy.sh
│   ├── install_autostart.sh
│   ├── probe_board.sh
│   ├── fix_static_ip_145.sh
│   └── apply_boot_demo.sh
├── build/
│   └── rk356x_demo
└── 技术栈说明.md
```

## 硬件与系统平台

- 开发板平台：Rockchip RK356x / RK3568
- CPU 架构：ARM64 / AArch64
- 系统环境：嵌入式 Linux
- 显示设备：LCD 屏幕，通过 `/dev/fb0` framebuffer 直接绘制
- 触摸设备：Goodix 触摸屏，通过 `/dev/input/event*` 输入事件读取坐标
- 摄像头：USB 摄像头，通过 V4L2 接口读取 `/dev/video9`
- 无线网络：`wlan0`，通过 `wpa_supplicant`、`wpa_cli`、`udhcpc` 连接 Wi-Fi 并获取 IP

## 开发语言与程序结构

- 主程序语言：C++17
- 构建工具：Makefile
- 程序入口：`src/main.cpp`
- 输出程序：`build/rk356x_demo`

程序主要模块：

- LCD 绘制模块：负责文字、按钮、图片和检测框绘制
- 触摸输入模块：读取触摸坐标并判断按钮点击、滑动操作
- 摄像头模块：使用 V4L2 打开摄像头、采集图像并转换为 RGB
- YOLO 推理模块：加载 RKNN 模型并执行目标检测
- Wi-Fi 管理模块：扫描热点、输入密码、连接网络、显示本机 IP
- 图库模块：保存抓拍图片、查看图片、切换图片、删除图片
- 自动抓拍模块：识别到 `person` 后自动保存带检测框的图片

## 图形界面技术

本项目没有使用 Qt、GTK 或 Web 前端，而是直接操作 Linux framebuffer。

基本流程：

1. 打开 `/dev/fb0`
2. 使用 `ioctl` 获取屏幕分辨率、色深和行长度
3. 通过 `mmap` 将显存映射到用户态
4. 根据坐标计算像素地址
5. 直接写入 RGB 像素
6. 手动绘制文字、按钮、矩形、检测框和图片

文字绘制采用程序内置的 5x7 点阵字体；按钮、检测框和背景由矩形填充完成；摄像头画面和图库图片通过 RGB 像素缩放后写入 framebuffer。

## 人机交互技术

触摸屏输入基于 Linux input 子系统：

- 自动扫描 `/dev/input/event*`
- 判断设备是否支持 `EV_ABS`、`ABS_X`、`ABS_Y` 或多点触摸坐标
- 读取触摸按下、抬起事件
- 将原始坐标映射到 LCD 分辨率
- 根据坐标判断按钮点击区域
- 支持简单滑动判断，用于图库图片切换

主界面底部按钮：

- `OPEN`：打开摄像头
- `SNAP`：手动抓拍并保存带检测框图片
- `AUTO OFF / AUTO ON`：开启或关闭识别到人自动抓拍
- `CLOSE`：关闭摄像头

右上角按钮：

- `GALLERY`：进入图库
- `WIFI`：进入 Wi-Fi 管理页面

## 摄像头采集技术

摄像头采集使用 V4L2。

主要流程：

1. 打开 `/dev/video9`
2. 查询摄像头能力
3. 设置图像格式和分辨率
4. 使用 mmap 申请采集缓冲区
5. 开启视频流
6. 循环读取图像帧
7. 将 YUYV / UYVY / NV12 转换为 RGB
8. 在 LCD 上实时预览

当前默认摄像头参数：

- 设备：`/dev/video9`
- 分辨率：640x480
- 格式：优先使用 YUYV

如需手动指定摄像头：

```sh
CAMERA_DEV=/dev/video9 ./build/rk356x_demo
```

## AI 推理与视觉识别

AI 推理使用 Rockchip RKNN Runtime。

主要组件：

- 推理库：`lib/librknnrt.so`
- 模型文件：`model/yolov5s_rk3568.rknn`
- 类别文件：`model/coco_80_labels_list.txt`
- 模型类型：YOLOv5 目标检测模型
- 推理后端：RK356x NPU / RKNN Runtime

推理流程：

1. 摄像头采集 RGB 图像
2. 将图像按 YOLO 输入尺寸缩放和 padding
3. 调用 RKNN Runtime 执行推理
4. 解析 YOLO 输出层
5. 执行置信度过滤和 NMS 去重
6. 得到类别、置信度和检测框坐标
7. 在实时画面和抓拍图片上绘制检测框

当前自动抓拍逻辑：

- 打开摄像头
- 点击 `AUTO OFF` 切换为 `AUTO ON`
- 当检测结果中包含 `person` 类别时自动保存图片
- 自动抓拍间隔限制为 5 秒，避免连续保存过多图片

## 图片保存与图库

抓拍图片保存为 BMP 格式。

保存目录：

```text
/root/rk356x_demo/gallery
```

图片内容：

- 原始摄像头画面
- YOLO 检测框
- 手动抓拍或自动抓拍生成的时间戳文件名

图库功能：

- 查看已抓拍图片
- `PREV` 上一张
- `NEXT` 下一张
- 左右滑动切换
- `DEL` 删除当前图片
- `BACK` 返回主界面

## Wi-Fi 管理

Wi-Fi 管理直接调用系统命令完成。

涉及工具：

- `iw`
- `iwlist`
- `wpa_supplicant`
- `wpa_cli`
- `udhcpc`
- `ip`

主要功能：

- 扫描附近 Wi-Fi
- 显示热点名称和信号强度
- 触摸屏输入密码
- 写入 `/etc/wpa_supplicant.conf`
- 启动 `wpa_supplicant`
- 通过 DHCP 获取 IP
- 在界面显示当前 `wlan0` IP

