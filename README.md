# iMagnetRest - 强大的Torrent下载管理器

一个使用Go语言开发的HTTP服务器，支持多种torrent下载方式和视频实时播放功能。

## 🚀 主要功能

### 📥 多种下载方式
- **Magnet链接下载** - 支持标准magnet链接
- **本地.torrent文件下载** - 支持本地torrent文件路径
- **HTTP .torrent文件下载** - 支持远程torrent文件URL
- **文件上传下载** - 支持拖拽上传.torrent文件

### 🎬 视频实时播放
- **边下载边播放** - 无需等待完整下载即可开始播放
- **Range请求支持** - 支持快进到任意位置
- **专业播放器** - 全屏播放、音量控制、键盘快捷键
- **多格式支持** - MP4、AVI、MKV、MOV、WMV、FLV、WebM等

### 📊 实时管理
- **下载进度监控** - 实时显示下载状态和进度
- **任务管理** - 支持取消下载和移除任务
- **文件管理** - 查看已下载文件，支持在线播放
- **Web界面** - 现代化响应式Web管理界面

## 🛠️ 技术栈

- **后端**: Go + Gin框架
- **Torrent库**: anacrolix/torrent
- **前端**: HTML5 + CSS3 + JavaScript
- **视频播放**: HTML5 Video + Range请求

## 📦 安装和运行

### 环境要求
- Go 1.19+
- 网络连接

### 快速开始

1. **克隆项目**
```bash
git clone <repository-url>
cd iMagnetRest
```

2. **安装依赖**
```bash
go mod tidy
```

3. **运行服务器**
```bash
go run .
```

4. **访问Web界面**
```
http://localhost:8080
```

## 🌐 API接口

### 下载管理
- `POST /download` - 统一下载接口
- `POST /upload` - 上传torrent文件
- `GET /status` - 获取下载状态
- `POST /cancel/:hash` - 取消下载
- `DELETE /remove/:hash` - 移除任务

### 文件服务
- `GET /files` - 获取文件列表
- `GET /stream/:filename` - 视频流式播放
- `GET /downloads/:filepath` - 文件下载

### Web界面
- `GET /` - 主页（重定向到Web界面）
- `GET /static/*` - 静态文件服务

## 📱 使用方法

### 1. Magnet链接下载
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"magnet_url": "magnet:?xt=urn:btih:..."}'
```

### 2. 本地torrent文件下载
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"torrent_file": "/path/to/file.torrent"}'
```

### 3. HTTP torrent文件下载
```bash
curl -X POST http://localhost:8080/download \
  -H "Content-Type: application/json" \
  -d '{"torrent_url": "http://example.com/file.torrent"}'
```

### 4. 上传torrent文件
```bash
curl -X POST http://localhost:8080/upload \
  -F 'torrent=@/path/to/file.torrent'
```

## 🎮 Web界面功能

### 仪表板
- 活跃下载数量统计
- 已下载文件总数
- 总下载量显示

### 下载管理
- 三种下载方式的标签页切换
- 拖拽上传支持
- 实时进度条显示
- 取消和移除按钮

### 视频播放
- 一键播放视频文件
- 专业播放器界面
- 支持快进、音量控制、全屏
- 键盘快捷键支持

## 🎯 键盘快捷键

在视频播放器中：
- `空格` - 播放/暂停
- `←/→` - 快退/快进10秒
- `↑/↓` - 音量调节
- `F` - 全屏切换
- `M` - 静音切换

## 📁 项目结构

```
iMagnetRest/
├── main.go                    # 主程序和路由设置
├── simple_torrent_service.go  # Torrent服务核心逻辑
├── stream_handler.go          # 视频流处理器
├── torrent_upload.go          # 文件上传处理
├── static/                    # Web界面文件
│   ├── index.html            # 主界面
│   └── video_player.html     # 视频播放器
├── downloads/                 # 下载文件存储目录
├── uploads/                   # 上传文件临时目录
├── go.mod                     # Go模块文件
└── README.md                  # 项目说明
```

## 🔧 配置说明

### 默认配置
- **端口**: 8080
- **下载目录**: ./downloads
- **上传目录**: ./uploads
- **允许上传**: 是（提高下载速度）
- **允许做种**: 是

### 自定义配置
可以通过修改 `simple_torrent_service.go` 中的配置来调整：
- 下载目录路径
- 网络设置
- 上传/下载限制

## 🚨 注意事项

1. **网络环境**: 需要良好的网络连接，某些地区可能需要代理
2. **防火墙**: 确保相关端口未被防火墙阻止
3. **磁盘空间**: 确保有足够的磁盘空间存储下载文件
4. **法律合规**: 请确保下载的内容符合当地法律法规

## 🤝 贡献

欢迎提交Issue和Pull Request来改进项目！

## 📄 许可证

本项目采用MIT许可证，详见LICENSE文件。

## 🙏 致谢

- [anacrolix/torrent](https://github.com/anacrolix/torrent) - 优秀的Go torrent库
- [gin-gonic/gin](https://github.com/gin-gonic/gin) - 高性能的Go web框架

---

**享受你的torrent下载和视频播放体验！** 🎉