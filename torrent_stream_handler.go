package main

import (
	"io"
	"log"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/gin-gonic/gin"
)

// 流式传输Torrent文件的指定范围 - 支持边下载边播放
func streamTorrentRange(c *gin.Context, torrentFile *torrent.File, start int64, contentLength int64) {
	buffer := make([]byte, 32768) // 32KB缓冲区
	remaining := contentLength
	currentPos := start

	log.Printf("开始流式传输Torrent文件: %s, 起始位置: %d, 长度: %d", torrentFile.Path(), start, contentLength)

	for remaining > 0 {
		// 计算本次读取的大小
		toRead := int64(len(buffer))
		if remaining < toRead {
			toRead = remaining
		}

		// 等待数据可用 - 智能缓冲策略
		if !waitForDataAvailable(torrentFile, currentPos, toRead) {
			log.Printf("等待数据超时，位置: %d", currentPos)
			break
		}

		// 创建读取器从指定位置读取
		reader := torrentFile.NewReader()
		reader.Seek(currentPos, io.SeekStart)

		// 读取数据
		n, err := reader.Read(buffer[:toRead])
		reader.Close()

		if err != nil && err != io.EOF {
			log.Printf("读取Torrent文件数据失败: %v", err)
			break
		}
		if n == 0 {
			break
		}

		// 写入响应并立即刷新
		if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
			log.Printf("写入响应失败: %v", writeErr)
			break
		}
		c.Writer.Flush()

		remaining -= int64(n)
		currentPos += int64(n)

		// 检查客户端是否断开连接
		select {
		case <-c.Request.Context().Done():
			log.Printf("客户端断开连接")
			return
		default:
		}
	}

	log.Printf("Torrent文件流式传输完成")
}

// 等待数据可用 - 智能缓冲策略
func waitForDataAvailable(torrentFile *torrent.File, position int64, length int64) bool {
	maxWaitTime := 30 * time.Second
	checkInterval := 500 * time.Millisecond
	startTime := time.Now()

	// 计算需要的piece范围
	pieceLength := torrentFile.Torrent().Info().PieceLength
	startPiece := int(position / pieceLength)
	endPiece := int((position + length - 1) / pieceLength)

	log.Printf("等待数据可用: 位置 %d-%d, piece %d-%d", position, position+length-1, startPiece, endPiece)

	for time.Since(startTime) < maxWaitTime {
		allAvailable := true

		// 检查所需的piece是否都已下载
		for pieceIndex := startPiece; pieceIndex <= endPiece; pieceIndex++ {
			if pieceIndex >= torrentFile.Torrent().NumPieces() {
				break
			}
			
			piece := torrentFile.Torrent().Piece(pieceIndex)
			if !piece.State().Complete {
				allAvailable = false
				// 优先下载这个piece
				piece.SetPriority(torrent.PiecePriorityHigh)
				log.Printf("等待piece %d下载完成...", pieceIndex)
				break
			}
		}

		if allAvailable {
			log.Printf("所需数据已可用")
			return true
		}

		time.Sleep(checkInterval)
	}

	log.Printf("等待数据超时")
	return false
}

// 流式传输整个Torrent文件 - 支持边下载边播放
func streamTorrentFile(c *gin.Context, torrentFile *torrent.File) {
	buffer := make([]byte, 32768) // 32KB缓冲区
	fileSize := torrentFile.Length()
	currentPos := int64(0)

	log.Printf("开始流式传输整个Torrent文件: %s, 大小: %d", torrentFile.Path(), fileSize)

	reader := torrentFile.NewReader()
	defer reader.Close()

	for currentPos < fileSize {
		// 计算本次读取的大小
		toRead := int64(len(buffer))
		if fileSize-currentPos < toRead {
			toRead = fileSize - currentPos
		}

		// 等待数据可用
		if !waitForDataAvailable(torrentFile, currentPos, toRead) {
			log.Printf("等待数据超时，位置: %d", currentPos)
			break
		}

		// 读取数据
		n, err := reader.Read(buffer[:toRead])
		if err != nil && err != io.EOF {
			log.Printf("读取Torrent文件数据失败: %v", err)
			break
		}
		if n == 0 {
			break
		}

		// 写入响应并立即刷新
		if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
			log.Printf("写入响应失败: %v", writeErr)
			break
		}
		c.Writer.Flush()

		currentPos += int64(n)

		// 检查客户端是否断开连接
		select {
		case <-c.Request.Context().Done():
			log.Printf("客户端断开连接")
			return
		default:
		}
	}

	log.Printf("Torrent文件流式传输完成")
}

// 预加载策略 - 为快进做准备
func preloadAroundPosition(torrentFile *torrent.File, position int64, windowSize int64) {
	pieceLength := torrentFile.Torrent().Info().PieceLength
	
	// 计算预加载窗口
	startPos := position - windowSize/2
	if startPos < 0 {
		startPos = 0
	}
	endPos := position + windowSize/2
	if endPos > torrentFile.Length() {
		endPos = torrentFile.Length()
	}

	startPiece := int(startPos / pieceLength)
	endPiece := int((endPos - 1) / pieceLength)

	log.Printf("预加载位置 %d 周围的数据, piece %d-%d", position, startPiece, endPiece)

	// 设置piece优先级
	for pieceIndex := startPiece; pieceIndex <= endPiece; pieceIndex++ {
		if pieceIndex >= torrentFile.Torrent().NumPieces() {
			break
		}
		
		piece := torrentFile.Torrent().Piece(pieceIndex)
		if !piece.State().Complete {
			piece.SetPriority(torrent.PiecePriorityHigh)
		}
	}
}

// 检查Torrent文件中的特定位置是否可播放
func isTorrentPositionPlayable(torrentFile *torrent.File, position int64, bufferSize int64) bool {
	pieceLength := torrentFile.Torrent().Info().PieceLength
	startPiece := int(position / pieceLength)
	endPiece := int((position + bufferSize - 1) / pieceLength)

	// 检查关键piece是否已下载
	for pieceIndex := startPiece; pieceIndex <= endPiece && pieceIndex < torrentFile.Torrent().NumPieces(); pieceIndex++ {
		piece := torrentFile.Torrent().Piece(pieceIndex)
		if !piece.State().Complete {
			return false
		}
	}

	return true
}