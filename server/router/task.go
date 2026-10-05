package router

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"

	"bilidown/task"
	"bilidown/util"
)

func createTask(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if r.Method != http.MethodPost {
		util.Res{Success: false, Message: "不支持的请求方法"}.Write(w)
		return
	}
	var body []task.TaskInDB
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		util.Res{Success: false, Message: "参数错误"}.Write(w)
		return
	}
	db := util.MustGetDB()
	defer db.Close()
	if folder, ferr := util.GetCurrentFolder(db); ferr == nil {
		if derr := util.CheckDisk(folder, 0); derr != nil {
			util.Res{Success: false, Message: derr.Error()}.Write(w)
			return
		}
	}
	for _, item := range body {
		if !util.CheckBvidFormat(item.Bvid) {
			util.Res{Success: false, Message: "bvid 格式错误"}.Write(w)
			return
		}
		if item.Cover == "" || item.Title == "" || item.Owner == "" {
			util.Res{Success: false, Message: "参数错误"}.Write(w)
		}

		if !util.IsValidURL(item.Cover) {
			util.Res{Success: false, Message: "封面链接格式错误"}.Write(w)
			return
		}
		if !util.IsValidURL(item.Audio) {
			util.Res{Success: false, Message: "音频链接格式错误"}.Write(w)
			return
		}
		if !util.IsValidURL(item.Video) {
			util.Res{Success: false, Message: "视频链接格式错误"}.Write(w)
			return
		}
		if !util.IsValidFormatCode(item.Format) {
			util.Res{Success: false, Message: "清晰度代码错误"}.Write(w)
			return
		}
		item.Folder, err = util.GetCurrentFolder(db)
		item.Status = "waiting"
		if err != nil {
			util.Res{Success: false, Message: fmt.Sprintf("util.GetCurrentFolder: %v.", err)}.Write(w)
			return
		}
		_task := task.Task{TaskInDB: item}
		_task.Title = util.FilterFileName(_task.Title)
		err = _task.Create(db)
		if err != nil {
			util.Res{Success: false, Message: fmt.Sprintf("_task.Create: %v.", err)}.Write(w)
			return
		}
		go _task.Start()
	}
	util.Res{Success: true, Message: "创建成功"}.Write(w)
}

func getActiveTask(w http.ResponseWriter, r *http.Request) {
	util.Res{Success: true, Data: task.GlobalTaskList}.Write(w)
}

func getTaskList(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		util.Res{Success: false, Message: "参数错误"}.Write(w)
		return
	}
	db := util.MustGetDB()
	defer db.Close()
	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil {
		page = 0
	}
	pageSize, err := strconv.Atoi(r.FormValue("pageSize"))
	if err != nil {
		pageSize = 360
	}
	tasks, err := task.GetTaskList(db, page, pageSize)
	if err != nil {
		util.Res{Success: false, Message: err.Error()}.Write(w)
		return
	}
	for i := range tasks {
		if tasks[i].Status == "done" {
			if _, err := os.Stat(tasks[i].FilePath()); err != nil {
				tasks[i].FileGone = true
			}
		}
	}
	util.Res{Success: true, Message: "获取成功", Data: tasks}.Write(w)
}

// lookupDoneTask 根据请求中的 id 找到“已完成且文件仍在服务器上”的任务。
// 出错时已向客户端写入 HTTP 错误，返回 ok=false。
func lookupDoneTask(w http.ResponseWriter, r *http.Request) (*task.TaskInDB, bool) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return nil, false
	}
	db := util.MustGetDB()
	defer db.Close()
	t, err := task.GetTask(db, id)
	if err == sql.ErrNoRows {
		http.Error(w, "任务不存在", http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	if t.Status != "done" {
		http.Error(w, "任务尚未完成", http.StatusConflict)
		return nil, false
	}
	if _, err := os.Stat(t.FilePath()); err != nil {
		http.Error(w, "文件已不在服务器上（可能已下载到本机并清理）", http.StatusGone)
		return nil, false
	}
	return t, true
}

// fetchFile 把已完成任务的文件以附件形式传给浏览器，完整传输成功后删除服务器上的文件，
// 让磁盘空间很小的设备只做临时中转。传输中断（浏览器取消、网络断开）时文件会保留，可重新下载。
func fetchFile(w http.ResponseWriter, r *http.Request) {
	t, ok := lookupDoneTask(w, r)
	if !ok {
		return
	}
	path := t.FilePath()
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ext := ".mp4"
	if t.DownloadType == "audio" {
		ext = ".m4a"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": t.Title + ext}))

	// 不走 Range：整文件顺序发送，才能准确判断“是否完整传完”
	n, err := io.Copy(w, f)
	if err != nil || n != info.Size() {
		log.Printf("fetchFile: task %d 传输未完成 (%d/%d bytes, err=%v)，保留服务器文件", t.ID, n, info.Size(), err)
		return
	}
	if err := os.Remove(path); err != nil {
		log.Printf("fetchFile: task %d 删除服务器文件失败: %v", t.ID, err)
		return
	}
	log.Printf("fetchFile: task %d 已传输并清理 (%d bytes)", t.ID, n)
}

func deleteTask(w http.ResponseWriter, r *http.Request) {
	taskIDStr := r.FormValue("id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		util.Res{Success: false, Message: "参数错误"}.Write(w)
		return
	}
	db := util.MustGetDB()
	defer db.Close()

	_task, err := task.GetTask(db, taskID)
	if err == sql.ErrNoRows {
		util.Res{Success: true, Message: "数据库中没有该条记录，所以本次操作被忽略，可以算作成功。"}.Write(w)
		return
	}
	if err != nil {
		util.Res{Success: false, Message: fmt.Sprintf("task.GetTask: %v", err)}.Write(w)
		return
	}
	filePath := _task.FilePath()
	err = os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		util.Res{Success: false, Message: fmt.Sprintf("文件删除失败 os.Remove: %v", err)}.Write(w)
		return
	}

	err = task.DeleteTask(db, taskID)
	if err != nil {
		util.Res{Success: false, Message: fmt.Sprintf("task.DeleteTask: %v", err)}.Write(w)
		return
	}
	util.Res{Success: true, Message: "删除成功"}.Write(w)
}
