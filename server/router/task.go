package router

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		// 去掉非法字符，并限制长度：磁盘上的文件名还要加编号和后缀，整体不能超过 255 字节
		_task.Title = util.TruncateBytes(util.FilterFileName(_task.Title), 200)
		err = _task.Create(db)
		if err != nil {
			util.Res{Success: false, Message: fmt.Sprintf("_task.Create: %v.", err)}.Write(w)
			return
		}
		go _task.Start()
	}
	util.Res{Success: true, Message: "创建成功"}.Write(w)
}

// activeTaskFromRequest 按 id 参数找到内存中的任务（暂停/继续/取消用）
func activeTaskFromRequest(w http.ResponseWriter, r *http.Request) (*task.Task, bool) {
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		util.Res{Success: false, Message: "参数错误"}.Write(w)
		return nil, false
	}
	t := task.FindActiveTask(id)
	if t == nil {
		util.Res{Success: false, Message: "任务不存在或已结束"}.Write(w)
		return nil, false
	}
	return t, true
}

func pauseTask(w http.ResponseWriter, r *http.Request) {
	if t, ok := activeTaskFromRequest(w, r); ok {
		if !t.SetPaused(true) {
			util.Res{Success: false, Message: "当前状态无法暂停（可能正在合并或已结束）"}.Write(w)
			return
		}
		util.Res{Success: true, Message: "已暂停"}.Write(w)
	}
}

func resumeTask(w http.ResponseWriter, r *http.Request) {
	if t, ok := activeTaskFromRequest(w, r); ok {
		if !t.SetPaused(false) {
			util.Res{Success: false, Message: "当前状态无法继续"}.Write(w)
			return
		}
		util.Res{Success: true, Message: "已继续"}.Write(w)
	}
}

func cancelTask(w http.ResponseWriter, r *http.Request) {
	if t, ok := activeTaskFromRequest(w, r); ok {
		if !t.Cancel() {
			util.Res{Success: false, Message: "当前状态无法取消（可能正在合并或已结束）"}.Write(w)
			return
		}
		util.Res{Success: true, Message: "已取消"}.Write(w)
	}
}

// showFile 仅本机模式注册：在资源管理器/访达中定位已完成任务的文件
func showFile(w http.ResponseWriter, r *http.Request) {
	t, ok := lookupDoneTask(w, r)
	if !ok {
		return
	}
	path := t.FilePath()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,"+path)
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	_ = cmd.Start() // explorer 即使成功也常返回非 0，只管启动
	util.Res{Success: true, Message: "已打开"}.Write(w)
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
			if info, err := os.Stat(tasks[i].FilePath()); err != nil {
				tasks[i].FileGone = true
			} else {
				tasks[i].FileSize = info.Size()
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
