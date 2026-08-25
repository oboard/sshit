# 常用窗口管理快捷键

本页汇总可用于常见平铺窗口管理器配置的快捷键参考，便于在 `sshit` 工作区中快速查阅。实际可用按键取决于本机窗口管理器与用户配置；如果覆盖了默认绑定，请以本机配置为准。

> `SUPER` 通常指键盘上的 Meta / Windows 键。

## 窗口管理

| 快捷键 | 功能 | Dispatcher |
| --- | --- | --- |
| `SUPER + Q` | 关闭当前窗口 | `killactive` |
| `SUPER + F` | 全屏切换 | `fullscreen` |
| `SUPER + V` | 切换自由模式 | `togglefloating` |
| `SUPER + P` | 伪平铺模式切换 | `pseudo` |
| `SUPER + J` | 切换分割方向 | `togglesplit` |
| `SUPER + M` | 退出窗口管理器 | `exit` |

## 窗口移动

| 快捷键 | 功能 | Dispatcher |
| --- | --- | --- |
| `SUPER + 方向键` | 移动焦点 | `movefocus` |
| `SUPER + H / L` | 左右移动焦点 | `movefocus` |
| `SUPER + K / J` | 上下移动焦点 | `movefocus` |
| `SUPER + SHIFT + 方向键` | 移动窗口 | `movewindow` |
| `SUPER + 鼠标拖拽` | 移动窗口 | `movewindow` |

## 工作区

| 快捷键 | 功能 | Dispatcher |
| --- | --- | --- |
| `SUPER + 1–9` | 切换到工作区 1–9 | `workspace` |
| `SUPER + SHIFT + 1–9` | 移动窗口到工作区 1–9 | `movetoworkspace` |
| `SUPER + S` | 切换到特殊工作区 | `togglespecialworkspace` |
| `SUPER + SHIFT + S` | 移动窗口到特殊工作区 | `movetoworkspacesilent` |
| `SUPER + 鼠标滚轮` | 切换工作区 | `workspace` |

## 应用启动

| 快捷键 | 功能 | Dispatcher |
| --- | --- | --- |
| `SUPER + Return` | 打开终端 | `exec [terminal]` |
| `SUPER + E` | 打开文件管理器 | `exec [filemanager]` |
| `SUPER + R` | 运行命令 | `exec [launcher]` |

## 媒体控制键

| 快捷键 | 功能 |
| --- | --- |
| `XF86AudioRaiseVolume` | 音量增加 5% |
| `XF86AudioLowerVolume` | 音量减少 5% |
| `XF86AudioMute` | 静音切换 |
| `XF86AudioPlay` | 播放 / 暂停 |
| `XF86AudioPrev` | 上一曲 |
| `XF86AudioNext` | 下一曲 |
| `XF86MonBrightnessUp` | 亮度增加 |
| `XF86MonBrightnessDown` | 亮度减少 |

## 窗口分组（Group）

| 快捷键 | 功能 | Dispatcher |
| --- | --- | --- |
| `SUPER + G` | 切换分组 | `togglegroup` |
| `SUPER + TAB` | 切换到组内下一个窗口 | `changegroupactive` |
| `SUPER + SHIFT + TAB` | 切换到组内上一个窗口 | `changegroupactive` |

## 鼠标操作

| 操作 | 功能 |
| --- | --- |
| `SUPER + 左键拖拽` | 移动窗口 |
| `SUPER + 右键拖拽` | 调整窗口大小 |
| `SUPER + 鼠标滚轮` | 切换工作区 |

## sshit 工作区布局

浏览器工作区顶部右侧提供 **自由 / 平铺** 开关：

- **自由**：保留窗口的自由位置和尺寸，可从标题栏拖拽、使用右下角调整大小。
- **平铺**：将当前窗口排列到可调整比例的分割窗格中；切回自由会恢复原有的自由工作区。

界面过渡动画由 [Motion](https://motion.dev/) 驱动，以保持布局切换和控件反馈的轻盈、连贯。

### 平铺模式的快捷键与鼠标（Hyprland 风格）

平铺模式把键盘和鼠标统一到一个焦点模型上——无论按键还是点击，动作都作用于当前聚焦的那一格（pane），类似 Hyprland 的 `focusState`。本实现采用 `Ctrl` 作为主修饰键（浏览器里最不易与终端快捷键冲突）。

| 动作 | 快捷键 | 说明 |
| --- | --- | --- |
| 移动焦点 | `Ctrl + H / J / K / L`（或方向键） | 在平铺格间移动聚焦 pane |
| 交换窗口 | `Ctrl + Shift + 方向键` | 把焦点窗口与相邻窗口交换位置 |
| 切换分割方向 | `Ctrl + T` | 翻转焦点 pane 所在分割的轴向 |
| 关闭窗口 | `Ctrl + Q` | 关闭当前聚焦的 pane |
| 循环焦点 | `Ctrl + \`` | 在平铺 pane 间循环 |

**鼠标协同（含 `drag_threshold` 区分单击与拖动）：**

| 操作 | 功能 |
| --- | --- |
| 点击某个 pane | 设置平铺焦点（面板格间不重叠 z-order） |
| `Ctrl + 拖拽` pane | 让窗口跟随鼠标，并在经过其他 pane 时实时重排 |
| 松开鼠标 | 提交最终的窗格顺序 |

> 平铺布局是**房间内共享**的：窗格划分与 `Ctrl + 拖拽` 的最终排序通过工作区 WebSocket 由服务端同步和持久化；Yjs `/collab` 仅承载编辑器等协作文档。**自由 / 平铺的切换是本地的查看偏好**，不随共享布局切换，并保存在浏览器中以在刷新后恢复；自由窗口的 position/size 仍由服务端共享几何同步。
