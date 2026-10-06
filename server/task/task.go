package task

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"bilidown/bilibili"
	"bilidown/common"
	"bilidown/util"
)

// TaskInitOption 创建任务时需要从 POST 请求获取的参数
type TaskInitOption struct {
	Bvid         string             `json:"bvid"`
	Cid          int                `json:"cid"`
	Format       common.MediaFormat `json:"format"`
	Title        string             `json:"title"`
	Owner        string             `json:"owner"`
	Cover        string             `json:"cover"`
	Status       TaskStatus         `json:"status"`
	Folder       string             `json:"folder"`
	Audio        string             `json:"audio"`
	Video        string             `json:"video"`
	Duration     int                `json:"duration"`
	DownloadType string             `json:"downloadType"`
}

// TaskInDB 任务数据库中的数据
type TaskInDB struct {
	TaskInitOption
	ID       int64     `json:"id"`
	CreateAt time.Time `json:"createAt"`
	// FileGone 表示任务已完成但文件已不存在（例如被手动删除），仅在返回任务列表时计算。
	FileGone bool `json:"fileGone"`
	// FileSize 文件的真实大小（字节），仅在返回任务列表时计算。
	FileSize int64 `json:"fileSize"`
}

func (task *TaskInDB) FilePath() string {
	ext := ".mp4"
	if task.DownloadType == "audio" {
		ext = ".m4a"
	}
	return filepath.Join(task.Folder,
		fmt.Sprintf("%s %s%s", task.Title,
			strings.Replace(base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(task.ID, 10))), "=", "", -1),
			ext,
		),
	)
}

// done | waiting | running | error
type TaskStatus string

type Task struct {
	TaskInDB
	AudioProgress float64 `json:"audioProgress"`
	VideoProgress float64 `json:"videoProgress"`
	MergeProgress float64 `json:"mergeProgress"`
	// Paused 表示用户已暂停该任务（状态仍为 running/waiting）
	Paused bool `json:"paused"`

	ctlMu    sync.Mutex
	canceled bool
	semHeld  bool
	wake     chan struct{}
}

var GlobalTaskList = []*Task{}
var GlobalTaskMux = &sync.Mutex{}
var GlobalDownloadSem = util.NewSemaphore(3)
var GlobalMergeSem = util.NewSemaphore(3)

func (task *Task) Create(db *sql.DB) error {
	util.SqliteLock.Lock()
	result, err := db.Exec(`INSERT INTO "task" ("bvid", "cid", "format", "title", "owner", "cover", "status", "folder", "duration", "download_type")
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Bvid,
		task.Cid,
		task.Format,
		task.Title,
		task.Owner,
		task.Cover,
		task.Status,
		task.Folder,
		task.Duration,
		task.DownloadType,
	)
	util.SqliteLock.Unlock()
	if err != nil {
		return err
	}

	task.ID, err = result.LastInsertId()
	task.CreateAt = time.Now()
	return err
}

// Create 创建任务，并将任务加入全局任务列表
func (task *Task) Start() {
	if task.DownloadType == "" {
		task.DownloadType = "merge"
	}
	GlobalTaskMux.Lock()
	GlobalTaskList = append(GlobalTaskList, task)
	GlobalTaskMux.Unlock()
	db := util.MustGetDB()
	defer db.Close()
	sessdata, err := bilibili.GetSessdata(db)
	if err != nil {
		task.UpdateStatus(db, "error", fmt.Errorf("bilibili.GetSessdata: %v", err))
		return
	}
	client := &bilibili.BiliClient{SESSDATA: sessdata}

	// 任务没有成功完成时，清理遗留的临时文件，避免失败的任务白占磁盘
	defer func() {
		if task.Status != "done" {
			task.cleanupTemp()
		}
	}()

	task.acquireSem()
	if err := util.CheckDisk(task.Folder, 0); err != nil {
		task.releaseSem()
		task.UpdateStatus(db, "error", err)
		return
	}
	task.UpdateStatus(db, "running")

	if task.DownloadType == "audio" {
		// 仅音频模式：只下载音频，重命名音频文件为输出文件
		err = DownloadMedia(client, task.Audio, task, "audio")
		if err != nil {
			task.releaseSem()
			task.downloadFailed(db, err)
			return
		}
		task.releaseSem()
		outputPath := task.TaskInDB.FilePath()
		audioPath := filepath.Join(task.Folder, strconv.FormatInt(task.ID, 10)+".audio")
		err = os.Rename(audioPath, outputPath)
		if err != nil {
			task.UpdateStatus(db, "error", fmt.Errorf("os.Rename: %v", err))
			return
		}
		// 添加元数据
		if err := task.addMetadata(outputPath); err != nil {
			log.Printf("添加元数据失败 (任务ID: %d): %v", task.ID, err)
		}
		task.UpdateStatus(db, "done")
		return
	} else if task.DownloadType == "video" {
		// 仅视频模式：只下载视频，重命名视频文件为输出文件
		err = DownloadMedia(client, task.Video, task, "video")
		if err != nil {
			task.releaseSem()
			task.downloadFailed(db, err)
			return
		}
		task.releaseSem()
		outputPath := task.TaskInDB.FilePath()
		videoPath := filepath.Join(task.Folder, strconv.FormatInt(task.ID, 10)+".video")
		err = os.Rename(videoPath, outputPath)
		if err != nil {
			task.UpdateStatus(db, "error", fmt.Errorf("os.Rename: %v", err))
			return
		}
		// 添加元数据
		if err := task.addMetadata(outputPath); err != nil {
			log.Printf("添加元数据失败 (任务ID: %d): %v", task.ID, err)
		}
		task.UpdateStatus(db, "done")
		return
	} else {
		// 合并模式：下载音频和视频，然后合并
		err = DownloadMedia(client, task.Audio, task, "audio")
		if err != nil {
			task.releaseSem()
			task.downloadFailed(db, err)
			return
		}
		err = DownloadMedia(client, task.Video, task, "video")
		if err != nil {
			task.releaseSem()
			task.downloadFailed(db, err)
			return
		}
		task.releaseSem()

		outputPath := task.TaskInDB.FilePath()
		videoPath := filepath.Join(task.Folder, strconv.FormatInt(task.ID, 10)+".video")
		audioPath := filepath.Join(task.Folder, strconv.FormatInt(task.ID, 10)+".audio")
		GlobalMergeSem.Acquire()
		err = task.MergeMedia(outputPath, videoPath, audioPath)
		if err != nil {
			GlobalMergeSem.Release()
			task.UpdateStatus(db, "error", fmt.Errorf("task.MergeMedia: %v", err))
			return
		}
		err = os.Remove(videoPath)
		if err != nil {
			GlobalMergeSem.Release()
			task.UpdateStatus(db, "error", fmt.Errorf("os.Remove: %v", err))
			return
		}
		err = os.Remove(audioPath)
		if err != nil {
			GlobalMergeSem.Release()
			task.UpdateStatus(db, "error", fmt.Errorf("os.Remove: %v", err))
			return
		}
		GlobalMergeSem.Release()
		// 添加元数据
		if err := task.addMetadata(outputPath); err != nil {
			log.Printf("添加元数据失败 (任务ID: %d): %v", task.ID, err)
		}
		task.UpdateStatus(db, "done")
	}
}

// cleanupTemp 删除该任务可能遗留的 .audio / .video / 元数据临时文件
func (task *Task) cleanupTemp() {
	id := strconv.FormatInt(task.ID, 10)
	for _, p := range []string{
		filepath.Join(task.Folder, id+".audio"),
		filepath.Join(task.Folder, id+".video"),
		task.TaskInDB.FilePath() + ".tmp.mp4",
	} {
		_ = os.Remove(p)
	}
}

// 合并音视频
func (task *Task) MergeMedia(outputPath string, inputPaths ...string) error {
	inputs := []string{}
	for _, path := range inputPaths {
		inputs = append(inputs, "-i", path)
	}

	ffmpegPath, err := util.GetFFmpegPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(ffmpegPath, append(inputs, "-c:v", "copy", "-c:a", "copy", "-progress", "pipe:1", "-strict", "-2", outputPath)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)

	progress := newProgressBar(int64(task.Duration))
	outTimeRegex := regexp.MustCompile(`out_time_ms=(\d+)`) // 毫秒

	for scanner.Scan() {
		line := scanner.Text()
		match := outTimeRegex.FindStringSubmatch(line)
		if len(match) == 2 {
			outTime, err := strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				return err
			}
			progress.current = outTime / 1000000
			task.MergeProgress = progress.percent()
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	task.MergeProgress = 1
	return nil
}

func GetVideoURL(medias []bilibili.Media, format common.MediaFormat) (string, error) {
	for _, code := range []int{12, 7, 13} {
		for _, item := range medias {
			if item.ID == format && item.Codecid == code {
				return item.BaseURL, nil
			}
		}
	}
	return "", errors.New("未找到对应视频分辨率格式")
}

func GetAudioURL(dash *bilibili.Dash) string {
	if dash.Flac != nil {
		return dash.Flac.Audio.BaseURL
	}
	var maxAudioID common.MediaFormat
	var audioURL string
	for _, item := range dash.Audio {
		if item.ID > maxAudioID {
			maxAudioID = item.ID
			audioURL = item.BaseURL
		}
	}
	return audioURL
}

func (task *Task) UpdateStatus(db *sql.DB, status TaskStatus, errs ...error) error {
	util.SqliteLock.Lock()
	_, err := db.Exec(`UPDATE "task" SET "status" = ? WHERE "id" = ?`, status, task.ID)
	util.SqliteLock.Unlock()
	if err != nil {
		return nil
	}
	for _, err := range errs {
		if err != nil {
			err = util.CreateLog(db, fmt.Sprintf("Task-%d-Error: %v", task.ID, err))
			if err != nil {
				log.Fatalln("CreateLog:", err)
			}
		}
	}
	task.Status = status
	return err
}

// ErrCanceled 表示任务被用户取消
var ErrCanceled = errors.New("任务已取消")

// acquireSem 取得下载名额；已持有时直接返回
func (task *Task) acquireSem() {
	task.ctlMu.Lock()
	held := task.semHeld
	task.ctlMu.Unlock()
	if held {
		return
	}
	GlobalDownloadSem.Acquire()
	task.ctlMu.Lock()
	task.semHeld = true
	task.ctlMu.Unlock()
}

// releaseSem 归还下载名额；重复调用是安全的
func (task *Task) releaseSem() {
	task.ctlMu.Lock()
	held := task.semHeld
	task.semHeld = false
	task.ctlMu.Unlock()
	if held {
		GlobalDownloadSem.Release()
	}
}

// wakeAll 唤醒正在等待继续的下载（需持有 ctlMu）
func (task *Task) wakeAll() {
	if task.wake != nil {
		close(task.wake)
		task.wake = nil
	}
}

// SetPaused 暂停或继续任务。只有排队中/下载中的任务可以暂停，返回是否生效。
func (task *Task) SetPaused(paused bool) bool {
	task.ctlMu.Lock()
	defer task.ctlMu.Unlock()
	if task.Status != "running" && task.Status != "waiting" {
		return false
	}
	if task.canceled || task.MergeProgress > 0 {
		return false
	}
	task.Paused = paused
	task.wakeAll()
	return true
}

// Cancel 取消任务：停止下载、删除临时文件和任务记录
func (task *Task) Cancel() bool {
	task.ctlMu.Lock()
	defer task.ctlMu.Unlock()
	if task.Status != "running" && task.Status != "waiting" {
		return false
	}
	if task.MergeProgress > 0 {
		return false
	}
	task.canceled = true
	task.Paused = false
	task.wakeAll()
	return true
}

func (task *Task) ctlState() (paused, canceled bool) {
	task.ctlMu.Lock()
	defer task.ctlMu.Unlock()
	return task.Paused, task.canceled
}

// gate 在暂停期间让出下载名额并等待，继续后重新排队取得名额。
func (task *Task) gate() error {
	for {
		task.ctlMu.Lock()
		if task.canceled {
			task.ctlMu.Unlock()
			return ErrCanceled
		}
		if !task.Paused {
			task.ctlMu.Unlock()
			task.acquireSem()
			return nil
		}
		if task.wake == nil {
			task.wake = make(chan struct{})
		}
		ch := task.wake
		task.ctlMu.Unlock()
		task.releaseSem()
		<-ch
	}
}

func (task *Task) downloadFailed(db *sql.DB, err error) {
	if errors.Is(err, ErrCanceled) {
		task.finishCanceled(db)
		return
	}
	task.UpdateStatus(db, "error", fmt.Errorf("DownloadMedia: %v", err))
}

// finishCanceled 清理被取消的任务：删临时文件、删记录、从内存列表移除
func (task *Task) finishCanceled(db *sql.DB) {
	task.cleanupTemp()
	_ = DeleteTask(db, int(task.ID))
	GlobalTaskMux.Lock()
	for i, t := range GlobalTaskList {
		if t == task {
			GlobalTaskList = append(GlobalTaskList[:i], GlobalTaskList[i+1:]...)
			break
		}
	}
	GlobalTaskMux.Unlock()
}

// FindActiveTask 在内存任务列表中按 ID 查找
func FindActiveTask(id int64) *Task {
	GlobalTaskMux.Lock()
	defer GlobalTaskMux.Unlock()
	for _, t := range GlobalTaskList {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// DownloadMedia 下载一路媒体流。支持暂停/继续（HTTP Range 断点续传）、取消，
// 以及网络中断后从断点自动重试。
func DownloadMedia(client *bilibili.BiliClient, _url string, task *Task, mediaType string) error {
	path := filepath.Join(task.Folder, strconv.FormatInt(task.ID, 10)+"."+mediaType)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	var offset, total int64
	fails := 0
	setProgress := func() {
		if total <= 0 {
			return
		}
		GlobalTaskMux.Lock()
		if mediaType == "video" {
			task.VideoProgress = float64(offset) / float64(total)
		} else {
			task.AudioProgress = float64(offset) / float64(total)
		}
		GlobalTaskMux.Unlock()
	}

	for {
		if err := task.gate(); err != nil {
			return err
		}
		before := offset
		resp, err := client.RangeGET(_url, offset)
		if err != nil {
			if fails++; fails > 5 {
				return err
			}
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			if fails++; fails > 5 {
				return fmt.Errorf("下载失败，HTTP %d", resp.StatusCode)
			}
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode == http.StatusOK && offset > 0 {
			// 服务器不支持 Range，只能从头再来
			if err := file.Truncate(0); err != nil {
				resp.Body.Close()
				return err
			}
			offset = 0
		}
		if resp.ContentLength > 0 {
			total = offset + resp.ContentLength
			// 合并阶段还会再占用一份空间，所以按剩余部分的 2 倍估算
			if err := util.CheckDisk(task.Folder, uint64(resp.ContentLength)*2); err != nil {
				resp.Body.Close()
				return err
			}
		}
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			resp.Body.Close()
			return err
		}

		interrupted := false
		buf := make([]byte, 32*1024)
		for {
			paused, canceled := task.ctlState()
			if canceled {
				resp.Body.Close()
				return ErrCanceled
			}
			if paused {
				interrupted = true
				break
			}
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := file.Write(buf[:n]); werr != nil {
					resp.Body.Close()
					return werr
				}
				offset += int64(n)
				setProgress()
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				interrupted = true
				if offset > before {
					fails = 0
				} else if fails++; fails > 5 {
					resp.Body.Close()
					return rerr
				}
				break
			}
		}
		resp.Body.Close()
		if interrupted {
			continue
		}
		if total > 0 && offset < total {
			// 连接被提前关闭：从断点继续
			if offset > before {
				fails = 0
			} else if fails++; fails > 5 {
				return io.ErrUnexpectedEOF
			}
			continue
		}
		return nil
	}
}

type progressBar struct {
	total   int64
	current int64
}

func (p *progressBar) add(n int) {
	p.current += int64(n)
}

func (p *progressBar) percent() float64 {
	return float64(p.current) / float64(p.total)
}

func newProgressBar(total int64) *progressBar {
	return &progressBar{
		total: total,
	}
}

func GetTaskList(db *sql.DB, page int, pageSize int) ([]TaskInDB, error) {
	tasks := []TaskInDB{}
	util.SqliteLock.Lock()
	rows, err := db.Query(`SELECT
		"id", "bvid", "cid", "format", "title",
		"owner", "cover", "status", "folder", "duration", "download_type", "create_at"
	FROM "task" ORDER BY "id" DESC LIMIT ?, ?`,
		page*pageSize, pageSize,
	)
	util.SqliteLock.Unlock()
	if err != nil {
		return nil, err
	}

	createAt := ""

	for rows.Next() {
		task := TaskInDB{}
		err = rows.Scan(
			&task.ID,
			&task.Bvid,
			&task.Cid,
			&task.Format,
			&task.Title,
			&task.Owner,
			&task.Cover,
			&task.Status,
			&task.Folder,
			&task.Duration,
			&task.DownloadType,
			&createAt,
		)
		if err != nil {
			return nil, err
		}
		task.CreateAt, err = time.Parse("2006-01-02 15:04:05", createAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func DeleteTask(db *sql.DB, taskID int) error {
	util.SqliteLock.Lock()
	_, err := db.Exec(`DELETE FROM "task" WHERE "id" = ?`, taskID)
	util.SqliteLock.Unlock()
	return err
}

func GetTask(db *sql.DB, taskID int) (*TaskInDB, error) {
	task := TaskInDB{}
	createAt := ""
	util.SqliteLock.Lock()
	err := db.QueryRow(`SELECT
		"id", "bvid", "cid", "format", "title",
		"owner", "cover", "status", "folder", "duration", "download_type", "create_at"
	FROM "task" WHERE "id" = ?`,
		taskID,
	).Scan(
		&task.ID,
		&task.Bvid,
		&task.Cid,
		&task.Format,
		&task.Title,
		&task.Owner,
		&task.Cover,
		&task.Status,
		&task.Folder,
		&task.Duration,
		&task.DownloadType,
		&createAt,
	)
	util.SqliteLock.Unlock()
	if err != nil {
		return nil, err
	}

	task.CreateAt, err = time.Parse("2006-01-02 15:04:05", createAt)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// addMetadata 使用 ffmpeg 给输出文件添加元数据（description 和 artist）
func (task *Task) addMetadata(filePath string) error {
	ffmpegPath, err := util.GetFFmpegPath()
	if err != nil {
		return err
	}

	desc := task.Bvid
	if desc == "" {
		desc = ""
	}

	author := task.Owner

	// 临时文件加上 .mp4 扩展名
	tempPath := filePath + ".tmp.mp4"

	// 使用双引号包裹文件路径，避免特殊字符
	cmd := exec.Command(ffmpegPath,
		"-i", filePath,
		"-metadata", "description="+desc,
		"-metadata", "artist="+author,
		"-codec", "copy",
		"-y",
		tempPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg添加元数据失败: %v, 输出: %s", err, string(output))
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("删除原文件失败: %v", err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("重命名临时文件失败: %v", err)
	}

	return nil
}