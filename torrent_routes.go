package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// 处理Torrent流播放请求
func handleTorrentStream(c *gin.Context, ts *SimpleTorrentService, requestPath string) {
	// 解析路径: torrent/{hash}/{filename}
	parts := strings.Split(requestPath, "/")
	if len(parts) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的torrent流路径"})
		return
	}

	hash := parts[1]
	filename := strings.Join(parts[2:], "/") // 支持子目录

	// URL解码文件名
	decodedFilename, err := url.QueryUnescape(filename)
	if err != nil {
		log.Printf("URL解码失败: %v", err)
		decodedFilename = filename
	}

	log.Printf("尝试流播放Torrent文件: hash=%s, filename=%s", hash, decodedFilename)

	// 获取torrent文件对象
	torrentFile, err := ts.GetTorrentFileByPath(hash, decodedFilename)
	if err != nil {
		log.Printf("获取Torrent文件失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Torrent文件不存在",
			"hash": hash,
			"filename": decodedFilename,
		})
		return
	}

	// 检查是否为视频文件
	if !isVideoFile(decodedFilename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不是视频文件"})
		return
	}

	// 获取文件大小
	fileSize := torrentFile.Length()
	log.Printf("找到Torrent视频文件: %s, 大小: %d bytes", decodedFilename, fileSize)

	// 设置基本响应头
	c.Header("Content-Type", getContentType(decodedFilename))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")

	// 预加载文件开头部分以确保可播放
	preloadAroundPosition(torrentFile, 0, 5*1024*1024) // 预加载前5MB

	// 处理Range请求（支持快进和断点续传）
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		// 解析Range请求的位置，预加载该位置周围的数据
		if strings.Contains(rangeHeader, "bytes=") {
			ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
			if len(ranges) >= 1 && ranges[0] != "" {
				if start, parseErr := parseRangeStart(ranges[0]); parseErr == nil {
					preloadAroundPosition(torrentFile, start, 10*1024*1024) // 预加载10MB窗口
				}
			}
		}
		handleTorrentRangeRequest(c, torrentFile, rangeHeader)
	} else {
		// 普通请求，返回整个文件
		c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
		c.Status(http.StatusOK)
		streamTorrentFile(c, torrentFile)
	}
}

// 解析Range起始位置
func parseRangeStart(rangeStart string) (int64, error) {
	if rangeStart == "" {
		return 0, nil
	}
	return parseInt64(rangeStart)
}

// 安全的int64解析
func parseInt64(s string) (int64, error) {
	var result int64
	var err error
	
	// 简单的字符串到int64转换
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + int64(char-'0')
		} else {
			return 0, fmt.Errorf("无效的数字: %s", s)
		}
	}
	
	return result, err
}

// 设置获取正在下载视频文件的API路由
func setupTorrentRoutes(r *gin.Engine, ts *SimpleTorrentService) {
	// 获取正在下载的视频文件列表
	r.GET("/downloading-videos", func(c *gin.Context) {
		videoFiles := ts.GetDownloadingVideoFiles()
		c.JSON(http.StatusOK, gin.H{
			"downloading_videos": videoFiles,
		})
	})

	// 获取特定torrent的文件列表
	r.GET("/torrent/:hash/files", func(c *gin.Context) {
		hash := c.Param("hash")
		files, err := ts.GetTorrentFiles(hash)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var fileInfos []gin.H
		for _, file := range files {
			// 检查文件是否可播放
			bufferSize := int64(1024 * 1024) // 1MB
			if file.Length() < bufferSize {
				bufferSize = file.Length()
			}
			playable := isTorrentPositionPlayable(file, 0, bufferSize)

			fileInfos = append(fileInfos, gin.H{
				"path":        file.Path(),
				"size":        file.Length(),
				"downloaded":  file.BytesCompleted(),
				"progress":    float64(file.BytesCompleted()) / float64(file.Length()) * 100,
				"is_video":    isVideoFile(file.Path()),
				"playable":    playable,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"hash":  hash,
			"files": fileInfos,
		})
	})
}