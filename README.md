# Perth
a local music player using TUI 
terminal-music-player/
├── go.mod
├── main.go                    # 启动程序：加载 UI（不再临时测试功能）
│
├── player/                    # 播放控制逻辑
│   ├── player.go              # 播放/暂停/停止/进度控制
│   └── decoder.go             # 解码音频格式
│
├── playlist/                  # 歌曲列表与元数据
│   ├── scanner.go             # 扫描目录
│   └── track.go               # Track 结构定义
│
├── ui/                        # UI 层（基于 Bubble Tea）
│   ├── model.go               # UI model 结构体及 Init/Update/View
│   ├── layout.go              # 用 Lip Gloss 编写的组件布局
│   ├── keymap.go              # 快捷键定义（抽离按键逻辑）
│   └── style.go               # 样式定义（颜色、边框、字体等）
│
├── util/
│   └── timefmt.go             # 工具函数：格式化时间等
│
├── internal/                  # 可选：非导出逻辑/状态机/控制器
│   └── controller.go          # UI 调用的统一接口层（如 Player + Playlist 组合）
│
├── assets/                    # 测试音乐文件（添加 .gitignore）
└── README.md




fix github commit problem