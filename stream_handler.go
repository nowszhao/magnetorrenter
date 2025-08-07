package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/anacrolix/torrent"
)

// 处理Range请求 - 支持边下载边播放
func handleRangeRequest(c *gin.Context, file *os.File, fileSize int64, rangeHeader string) {
	// 解析Range头部 (例如: "bytes=0-1023" 或 "bytes=1024-")
	ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(ranges) != 2 {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var start, end int64
	var err error

	// 解析起始位置
	if ranges[0] != "" {
		start, err = strconv.ParseInt(ranges[0], 10, 64)
		if err != nil || start < 0 {
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	// 解析结束位置
	if ranges[1] != "" {
		end, err = strconv.ParseInt(ranges[1], 10, 64)
		if err != nil || end >= fileSize {
			end = fileSize - 1
		}
	} else {
		end = fileSize - 1
	}

	// 验证范围
	if start > end || start >= fileSize {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// 计算内容长度
	contentLength := end - start + 1

	// 设置206 Partial Content响应头
	c.Status(http.StatusPartialContent)
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))

	// 定位到起始位置
	file.Seek(start, 0)

	// 流式传输指定范围的数据
	streamRange(c, file, contentLength)
}

// 处理Torrent文件的Range请求 - 支持边下载边播放
func handleTorrentRangeRequest(c *gin.Context, torrentFile *torrent.File, rangeHeader string) {
	fileSize := torrentFile.Length()
	
	// 解析Range头部
	ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(ranges) != 2 {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var start, end int64
	var err error

	// 解析起始位置
	if ranges[0] != "" {
		start, err = strconv.ParseInt(ranges[0], 10, 64)
		if err != nil || start < 0 {
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	// 解析结束位置
	if ranges[1] != "" {
		end, err = strconv.ParseInt(ranges[1], 10, 64)
		if err != nil || end >= fileSize {
			end = fileSize - 1
		}
	} else {
		end = fileSize - 1
	}

	// 验证范围
	if start > end || start >= fileSize {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// 计算内容长度
	contentLength := end - start + 1

	// 设置206 Partial Content响应头
	c.Status(http.StatusPartialContent)
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))

	// 流式传输Torrent文件的指定范围
	streamTorrentRange(c, torrentFile, start, contentLength)
}

// 流式传输文件范围
func streamRange(c *gin.Context, file *os.File, contentLength int64) {
	buffer := make([]byte, 32768) // 32KB缓冲区，适合视频流
	remaining := contentLength

	for remaining > 0 {
		toRead := int64(len(buffer))
		if remaining < toRead {
			toRead = remaining
		}

		n, err := file.Read(buffer[:toRead])
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}

		// 写入响应并立即刷新
		if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
			break
		}
		c.Writer.Flush()
		
		remaining -= int64(n)
	}
}

// 流式传输整个文件
func streamFile(c *gin.Context, file *os.File, fileSize int64) {
	buffer := make([]byte, 32768) // 32KB缓冲区

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}

		// 写入响应并立即刷新
		if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
			break
		}
		c.Writer.Flush()
	}
}