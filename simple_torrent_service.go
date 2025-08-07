package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// 数据结构定义
type DownloadStatus struct {
	ActiveDownloads int           `json:"active_downloads"`
	Torrents        []TorrentInfo `json:"torrents"`
}

type TorrentInfo struct {
	Name       string  `json:"name"`
	Progress   float64 `json:"progress"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Speed      int64   `json:"speed"`
	Status     string  `json:"status"`
	Hash       string  `json:"hash"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

type SimpleTorrentService struct {
	client      *torrent.Client
	downloadDir string
	torrents    map[string]*TorrentStatus
	mutex       sync.RWMutex
}

type TorrentStatus struct {
	Torrent     *torrent.Torrent
	Name        string
	Status      string
	Progress    float64
	Downloaded  int64
	Total       int64
	AddedTime   time.Time
}

func NewSimpleTorrentService(downloadDir string) *SimpleTorrentService {
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = downloadDir
	cfg.NoUpload = false
	cfg.Seed = true

	// 确保下载目录存在
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Fatal("创建下载目录失败:", err)
	}

	client, err := torrent.NewClient(cfg)
	if err != nil {
		log.Fatal("创建torrent客户端失败:", err)
	}

	log.Printf("Torrent客户端已创建，下载目录: %s", downloadDir)

	return &SimpleTorrentService{
		client:      client,
		downloadDir: downloadDir,
		torrents:    make(map[string]*TorrentStatus),
	}
}

func (sts *SimpleTorrentService) DownloadMagnet(magnetURL string) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

	// 添加torrent
	t, err := sts.client.AddMagnet(magnetURL)
	if err != nil {
		return fmt.Errorf("添加magnet链接失败: %v", err)
	}

	hash := t.InfoHash().String()
	
	// 立即存储状态
	sts.torrents[hash] = &TorrentStatus{
		Torrent:   t,
		Name:      "获取种子信息中...",
		Status:    "连接中",
		Progress:  0,
		Downloaded: 0,
		Total:     0,
		AddedTime: time.Now(),
	}

	log.Printf("添加magnet链接成功: %s", hash[:8])

	// 异步处理
	go sts.handleTorrent(t, hash)

	return nil
}

func (sts *SimpleTorrentService) DownloadTorrentFile(torrentPath string) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

	// 添加torrent文件
	t, err := sts.client.AddTorrentFromFile(torrentPath)
	if err != nil {
		return fmt.Errorf("添加torrent文件失败: %v", err)
	}

	hash := t.InfoHash().String()
	
	// 立即存储状态
	sts.torrents[hash] = &TorrentStatus{
		Torrent:   t,
		Name:      "读取种子文件中...",
		Status:    "处理中",
		Progress:  0,
		Downloaded: 0,
		Total:     0,
		AddedTime: time.Now(),
	}

	log.Printf("添加torrent文件成功: %s", hash[:8])

	// 异步处理
	go sts.handleTorrent(t, hash)

	return nil
}

func (sts *SimpleTorrentService) DownloadTorrentFromURL(torrentURL string) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

	// 从URL下载torrent文件
	resp, err := http.Get(torrentURL)
	if err != nil {
		return fmt.Errorf("下载torrent文件失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("下载torrent文件失败，状态码: %d", resp.StatusCode)
	}

	// 读取torrent数据并保存到临时文件
	torrentData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取torrent数据失败: %v", err)
	}

	// 创建临时文件
	tempFile := filepath.Join(sts.downloadDir, "temp.torrent")
	if err := os.WriteFile(tempFile, torrentData, 0644); err != nil {
		return fmt.Errorf("保存临时torrent文件失败: %v", err)
	}

	// 添加torrent文件
	t, err := sts.client.AddTorrentFromFile(tempFile)
	if err != nil {
		os.Remove(tempFile) // 清理临时文件
		return fmt.Errorf("添加torrent失败: %v", err)
	}

	hash := t.InfoHash().String()
	
	// 立即存储状态
	sts.torrents[hash] = &TorrentStatus{
		Torrent:   t,
		Name:      "处理远程种子文件中...",
		Status:    "处理中",
		Progress:  0,
		Downloaded: 0,
		Total:     0,
		AddedTime: time.Now(),
	}

	log.Printf("添加远程torrent文件成功: %s", hash[:8])

	// 异步处理
	go sts.handleTorrent(t, hash)

	// 清理临时文件
	go func() {
		time.Sleep(5 * time.Second)
		os.Remove(tempFile)
	}()

	return nil
}

func (sts *SimpleTorrentService) handleTorrent(t *torrent.Torrent, hash string) {
	// 等待种子信息，但设置超时
	select {
	case <-t.GotInfo():
		sts.mutex.Lock()
		if status, exists := sts.torrents[hash]; exists {
			status.Name = t.Name()
			status.Status = "开始下载"
			status.Total = t.Length()
		}
		sts.mutex.Unlock()
		
		log.Printf("获取到种子信息: %s, 文件数: %d", t.Name(), len(t.Files()))
		
		// 打印文件列表
		for i, file := range t.Files() {
			log.Printf("文件 %d: %s (大小: %d bytes)", i, file.Path(), file.Length())
		}
		
		// 下载所有文件
		t.DownloadAll()
		sts.monitorProgress(t, hash)
		
	case <-time.After(30 * time.Second):
		sts.mutex.Lock()
		if status, exists := sts.torrents[hash]; exists {
			status.Status = "获取信息超时"
		}
		sts.mutex.Unlock()
		log.Printf("获取种子信息超时: %s", hash[:8])
	}
}

func (sts *SimpleTorrentService) monitorProgress(t *torrent.Torrent, hash string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sts.mutex.Lock()
			if status, exists := sts.torrents[hash]; exists {
				status.Downloaded = t.BytesCompleted()
				if status.Total > 0 {
					// 确保进度不超过100%
					progress := float64(status.Downloaded) / float64(status.Total) * 100
					status.Progress = math.Min(progress, 100.0)
				}
				
				if t.Complete.Bool() {
					status.Status = "下载完成"
					sts.mutex.Unlock()
					log.Printf("下载完成: %s", status.Name)
					
					// 列出实际下载的文件
					log.Printf("下载目录: %s", sts.downloadDir)
					for _, file := range t.Files() {
						fullPath := filepath.Join(sts.downloadDir, file.Path())
						if stat, err := os.Stat(fullPath); err == nil {
							log.Printf("已下载文件: %s (大小: %d bytes)", fullPath, stat.Size())
						} else {
							log.Printf("文件不存在: %s, 错误: %v", fullPath, err)
							// 尝试创建文件目录
							dir := filepath.Dir(fullPath)
							if err := os.MkdirAll(dir, 0755); err != nil {
								log.Printf("创建目录失败: %s, 错误: %v", dir, err)
							}
						}
					}
					return
				} else {
					status.Status = "下载中"
					// 显示下载进度详情和文件状态
					log.Printf("下载进度: %s - %.2f%% (%d/%d bytes)", 
						status.Name, status.Progress, status.Downloaded, status.Total)
					
					// 检查部分下载的文件
					for _, file := range t.Files() {
						if file.BytesCompleted() > 0 {
							fullPath := filepath.Join(sts.downloadDir, file.Path())
							log.Printf("部分下载: %s - %d/%d bytes", fullPath, file.BytesCompleted(), file.Length())
						}
					}
				}
			}
			sts.mutex.Unlock()

		case <-t.Closed():
			return
		}
	}
}

func (sts *SimpleTorrentService) GetDownloadStatus() DownloadStatus {
	sts.mutex.RLock()
	defer sts.mutex.RUnlock()

	var torrents []TorrentInfo
	activeDownloads := 0

	for hash, status := range sts.torrents {
		if status.Status != "下载完成" && status.Status != "已取消" {
			activeDownloads++
		}

		torrents = append(torrents, TorrentInfo{
			Name:       status.Name,
			Progress:   status.Progress,
			Downloaded: status.Downloaded,
			Total:      status.Total,
			Speed:      0,
			Status:     status.Status,
			Hash:       hash,
		})
	}

	return DownloadStatus{
		ActiveDownloads: activeDownloads,
		Torrents:        torrents,
	}
}

func (sts *SimpleTorrentService) GetDownloadedFiles() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(sts.downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && !filepath.HasPrefix(info.Name(), ".torrent") {
			relPath, _ := filepath.Rel(sts.downloadDir, path)
			files = append(files, FileInfo{
				Name: info.Name(),
				Size: info.Size(),
				Path: relPath,
			})
		}

		return nil
	})

	return files, err
}

func (sts *SimpleTorrentService) CancelDownload(hash string) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

	status, exists := sts.torrents[hash]
	if !exists {
		return fmt.Errorf("下载任务不存在")
	}

	// 停止torrent
	status.Torrent.Drop()
	
	// 更新状态
	status.Status = "已取消"
	
	log.Printf("取消下载: %s (%s)", status.Name, hash[:8])
	
	return nil
}

func (sts *SimpleTorrentService) RemoveDownload(hash string) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

	status, exists := sts.torrents[hash]
	if !exists {
		return fmt.Errorf("下载任务不存在")
	}

	// 停止torrent
	status.Torrent.Drop()
	
	// 从列表中移除
	delete(sts.torrents, hash)
	
	log.Printf("移除下载任务: %s (%s)", status.Name, hash[:8])
	
	return nil
}

func (sts *SimpleTorrentService) GetTorrentHash(name string) string {
	sts.mutex.RLock()
	defer sts.mutex.RUnlock()

	for hash, status := range sts.torrents {
		if status.Name == name {
			return hash
		}
	}
	return ""
}

// 获取正在下载的Torrent文件信息 - 支持边下载边播放
func (sts *SimpleTorrentService) GetTorrentFiles(hash string) ([]*torrent.File, error) {
	sts.mutex.RLock()
	defer sts.mutex.RUnlock()

	status, exists := sts.torrents[hash]
	if !exists {
		return nil, fmt.Errorf("torrent不存在")
	}

	if status.Torrent == nil {
		return nil, fmt.Errorf("torrent未初始化")
	}

	// 等待torrent信息可用
	select {
	case <-status.Torrent.GotInfo():
		return status.Torrent.Files(), nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("获取torrent信息超时")
	}
}

// 根据文件路径获取Torrent文件对象
func (sts *SimpleTorrentService) GetTorrentFileByPath(hash string, filePath string) (*torrent.File, error) {
	files, err := sts.GetTorrentFiles(hash)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.Path() == filePath {
			return file, nil
		}
	}

	return nil, fmt.Errorf("文件不存在: %s", filePath)
}

// 获取所有正在下载的视频文件 - 包含可播放状态
func (sts *SimpleTorrentService) GetDownloadingVideoFiles() []DownloadingVideoFile {
	sts.mutex.RLock()
	defer sts.mutex.RUnlock()

	var videoFiles []DownloadingVideoFile

	for hash, status := range sts.torrents {
		if status.Torrent == nil || status.Status == "下载完成" || status.Status == "已取消" {
			continue
		}

		// 检查是否已获取到torrent信息
		select {
		case <-status.Torrent.GotInfo():
			// 遍历torrent中的所有文件
			for _, file := range status.Torrent.Files() {
				if isVideoFile(file.Path()) {
					// 简化可播放判断逻辑 - 只要有一定的下载进度就认为可播放
					downloaded := file.BytesCompleted()
					fileSize := file.Length()
					progress := float64(downloaded) / float64(fileSize) * 100
					
					// 如果下载了至少5%或者5MB的数据，就认为可以开始播放
					minBytes := int64(5 * 1024 * 1024) // 5MB
					minProgress := 5.0 // 5%
					
					playable := (downloaded >= minBytes) || (progress >= minProgress && downloaded > 1024*1024) // 至少1MB
					
					log.Printf("视频文件: %s, 进度: %.2f%%, 已下载: %d bytes, 可播放: %v", 
						file.Path(), progress, downloaded, playable)
					
					videoFiles = append(videoFiles, DownloadingVideoFile{
						Hash:       hash,
						TorrentName: status.Name,
						FileName:   file.Path(),
						FileSize:   fileSize,
						Downloaded: downloaded,
						Progress:   progress,
						Playable:   playable,
						Status:     status.Status,
					})
				}
			}
		default:
			// torrent信息还未获取到
		}
	}

	return videoFiles
}

// 正在下载的视频文件信息
type DownloadingVideoFile struct {
	Hash        string  `json:"hash"`
	TorrentName string  `json:"torrent_name"`
	FileName    string  `json:"file_name"`
	FileSize    int64   `json:"file_size"`
	Downloaded  int64   `json:"downloaded"`
	Progress    float64 `json:"progress"`
	Playable    bool    `json:"playable"`
	Status      string  `json:"status"`
}

func (sts *SimpleTorrentService) Close() {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

	for _, status := range sts.torrents {
		status.Torrent.Drop()
	}

	sts.client.Close()
}
