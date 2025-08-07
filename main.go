package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	// 创建下载目录
	if err := os.MkdirAll("downloads", 0755); err != nil {
		log.Fatal("创建下载目录失败:", err)
	}

	// 创建Gin路由器
	r := gin.Default()

	// 创建torrent服务
	torrentService := NewSimpleTorrentService("downloads")

	// 设置路由
	setupRoutes(r, torrentService)
	setupUploadRoutes(r, torrentService)
	setupTorrentRoutes(r, torrentService)

	fmt.Println("服务器启动在端口 8080")
	fmt.Println("使用方法:")
	fmt.Println("POST /download - 下载magnet链接/torrent文件/torrent URL")
	fmt.Println("POST /upload - 上传torrent文件并下载")
	fmt.Println("GET /stream/:filename - 流式播放视频文件")
	fmt.Println("GET /files - 查看已下载的文件")

	log.Fatal(http.ListenAndServe(":8080", r))
}

func setupRoutes(r *gin.Engine, ts *SimpleTorrentService) {
	// 统一下载接口
	r.POST("/download", func(c *gin.Context) {
		var req struct {
			MagnetURL   string `json:"magnet_url"`
			TorrentFile string `json:"torrent_file"`
			TorrentURL  string `json:"torrent_url"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
			return
		}

		// 判断下载类型
		if req.MagnetURL != "" {
			// Magnet链接下载
			go func() {
				if err := ts.DownloadMagnet(req.MagnetURL); err != nil {
					log.Printf("Magnet下载失败: %v", err)
				}
			}()
			c.JSON(http.StatusOK, gin.H{
				"message": "开始下载Magnet链接",
				"type":    "magnet",
				"source":  req.MagnetURL,
			})
		} else if req.TorrentFile != "" {
			// 本地torrent文件下载
			go func() {
				if err := ts.DownloadTorrentFile(req.TorrentFile); err != nil {
					log.Printf("Torrent文件下载失败: %v", err)
				}
			}()
			c.JSON(http.StatusOK, gin.H{
				"message": "开始下载Torrent文件",
				"type":    "file",
				"source":  req.TorrentFile,
			})
		} else if req.TorrentURL != "" {
			// HTTP torrent文件下载
			go func() {
				if err := ts.DownloadTorrentFromURL(req.TorrentURL); err != nil {
					log.Printf("远程Torrent文件下载失败: %v", err)
				}
			}()
			c.JSON(http.StatusOK, gin.H{
				"message": "开始下载远程Torrent文件",
				"type":    "url",
				"source":  req.TorrentURL,
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "请提供magnet_url、torrent_file或torrent_url中的一个",
			})
		}
	})

	// 流式播放视频 - 支持Range请求和实时播放，支持中文文件名和子目录，支持边下载边播放
	r.GET("/stream/*filepath", func(c *gin.Context) {
		// 获取完整的文件路径，去掉开头的斜杠
		requestPath := strings.TrimPrefix(c.Param("filepath"), "/")
		
		// 检查是否为torrent流播放请求 (格式: torrent/{hash}/{filename})
		if strings.HasPrefix(requestPath, "torrent/") {
			handleTorrentStream(c, ts, requestPath)
			return
		}
		
		// 原有的本地文件流播放逻辑
		filePath := filepath.Join("downloads", requestPath)
		log.Printf("尝试访问视频文件: %s", filePath)

		// 检查文件是否存在
		fileInfo, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			log.Printf("文件不存在: %s", filePath)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "文件不存在",
				"path": filePath,
				"requested": requestPath,
			})
			return
		}

		if err != nil {
			log.Printf("文件访问错误: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文件访问错误"})
			return
		}

		log.Printf("找到文件: %s, 大小: %d bytes", filePath, fileInfo.Size())

		// 检查是否为视频文件
		if !isVideoFile(filepath.Base(filePath)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不是视频文件"})
			return
		}

		// 打开文件
		file, err := os.Open(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法打开文件"})
			return
		}
		defer file.Close()

		// 获取文件大小
		fileSize := fileInfo.Size()

		// 设置基本响应头
		c.Header("Content-Type", getContentType(filepath.Base(filePath)))
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "no-cache")

		// 处理Range请求（支持快进和断点续传）
		rangeHeader := c.GetHeader("Range")
		if rangeHeader != "" {
			handleRangeRequest(c, file, fileSize, rangeHeader)
		} else {
			// 普通请求，返回整个文件
			c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
			c.Status(http.StatusOK)
			streamFile(c, file, fileSize)
		}
	})

	// 获取文件列表
	r.GET("/files", func(c *gin.Context) {
		files, err := ts.GetDownloadedFiles()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件列表失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"files": files,
		})
	})

	// 获取下载状态
	r.GET("/status", func(c *gin.Context) {
		status := ts.GetDownloadStatus()
		c.JSON(http.StatusOK, status)
	})

	// 取消下载
	r.POST("/cancel/:hash", func(c *gin.Context) {
		hash := c.Param("hash")
		if err := ts.CancelDownload(hash); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "下载已取消"})
	})

	// 移除下载任务
	r.DELETE("/remove/:hash", func(c *gin.Context) {
		hash := c.Param("hash")
		if err := ts.RemoveDownload(hash); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "下载任务已移除"})
	})

	// 静态文件服务（用于直接访问下载的文件）- 支持文件下载
	r.GET("/downloads/*filepath", func(c *gin.Context) {
		requestPath := strings.TrimPrefix(c.Param("filepath"), "/")
		filePath := filepath.Join("downloads", requestPath)

		// 检查文件是否存在
		fileInfo, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			return
		}

		// 设置下载响应头
		fileName := filepath.Base(filePath)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// 发送文件
		c.File(filePath)
	})
	
	// 静态Web页面服务
	r.Static("/static", "./static")
	
	// 删除文件接口
	r.DELETE("/delete-file", func(c *gin.Context) {
		var req struct {
			FilePath string `json:"file_path"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
			return
		}

		if req.FilePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件路径不能为空"})
			return
		}

		// 构建完整的文件路径
		fullPath := filepath.Join("downloads", req.FilePath)
		
		// 安全检查：确保文件路径在downloads目录内
		absDownloads, _ := filepath.Abs("downloads")
		absFilePath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absFilePath, absDownloads) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的文件路径"})
			return
		}

		// 检查文件是否存在
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			return
		}

		// 删除文件
		if err := os.Remove(fullPath); err != nil {
			log.Printf("删除文件失败: %s, 错误: %v", fullPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件失败"})
			return
		}

		// 尝试删除空的父目录
		dir := filepath.Dir(fullPath)
		if dir != "downloads" {
			os.Remove(dir) // 忽略错误，因为目录可能不为空
		}

		log.Printf("文件已删除: %s", fullPath)
		c.JSON(http.StatusOK, gin.H{
			"message": "文件删除成功",
			"deleted_file": req.FilePath,
		})
	})

	// 主页重定向到Web界面
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/static/index.html")
	})
}

// 检查是否为视频文件
func isVideoFile(filename string) bool {
	ext := filepath.Ext(filename)
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v"}
	
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// 获取内容类型
func getContentType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	case ".webm":
		return "video/webm"
	case ".m4v":
		return "video/x-m4v"
	default:
		return "application/octet-stream"
	}
}