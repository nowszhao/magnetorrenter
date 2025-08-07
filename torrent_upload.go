package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// 添加文件上传功能
func setupUploadRoutes(r *gin.Engine, ts *SimpleTorrentService) {
	// 上传torrent文件
	r.POST("/upload", func(c *gin.Context) {
		// 单文件上传
		file, err := c.FormFile("torrent")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上传文件失败"})
			return
		}

		// 检查文件扩展名
		if filepath.Ext(file.Filename) != ".torrent" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "只支持.torrent文件"})
			return
		}

		// 创建上传目录
		uploadDir := "uploads"
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败"})
			return
		}

		// 保存文件
		dst := filepath.Join(uploadDir, file.Filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
			return
		}

		// 开始下载
		go func() {
			if err := ts.DownloadTorrentFile(dst); err != nil {
				fmt.Printf("上传的torrent文件下载失败: %v\n", err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{
			"message":  "torrent文件上传成功，开始下载",
			"filename": file.Filename,
			"size":     file.Size,
		})
	})
}