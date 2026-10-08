package task

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"bilidown/bilibili"
)

// slowRangeServer 提供支持 Range 的慢速下载，便于在中途暂停
func slowRangeServer(data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := 0
		if rg := r.Header.Get("Range"); rg != "" {
			var s int
			if _, err := fmt.Sscanf(rg, "bytes=%d-", &s); err == nil {
				start = s
			}
			w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(len(data)-1)+"/"+strconv.Itoa(len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(len(data)-start))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		}
		for i := start; i < len(data); i += 16 * 1024 {
			end := i + 16*1024
			if end > len(data) {
				end = len(data)
			}
			if _, err := w.Write(data[i:end]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
}

func newTestTask(t *testing.T) *Task {
	task := &Task{}
	task.ID = 7
	task.Folder = t.TempDir()
	task.Status = "running"
	return task
}

func TestDownloadPauseResume(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 16*1024*4) // 4MB
	srv := slowRangeServer(data)
	defer srv.Close()

	task := newTestTask(t)
	done := make(chan error, 1)
	go func() { done <- DownloadMedia(&bilibili.BiliClient{}, srv.URL+"/a", task, "audio") }()

	time.Sleep(150 * time.Millisecond)
	if !task.SetPaused(true) {
		t.Fatal("pause refused")
	}
	time.Sleep(200 * time.Millisecond)
	GlobalTaskMux.Lock()
	p1 := task.AudioProgress
	GlobalTaskMux.Unlock()
	time.Sleep(300 * time.Millisecond)
	GlobalTaskMux.Lock()
	p2 := task.AudioProgress
	GlobalTaskMux.Unlock()
	if p1 <= 0 || p1 >= 1 {
		t.Fatalf("expected partial progress while paused, got %v", p1)
	}
	if p1 != p2 {
		t.Fatalf("progress moved while paused: %v -> %v", p1, p2)
	}
	select {
	case err := <-done:
		t.Fatalf("finished while paused: %v", err)
	default:
	}

	task.SetPaused(false)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timeout after resume")
	}
	task.releaseSem()
	got, err := os.ReadFile(filepath.Join(task.Folder, "7.audio"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch after resume: got %d bytes want %d", len(got), len(data))
	}
}

func TestDownloadCancel(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 8*1024*1024)
	srv := slowRangeServer(data)
	defer srv.Close()

	task := newTestTask(t)
	done := make(chan error, 1)
	go func() { done <- DownloadMedia(&bilibili.BiliClient{}, srv.URL+"/a", task, "video") }()
	time.Sleep(100 * time.Millisecond)
	if !task.Cancel() {
		t.Fatal("cancel refused")
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrCanceled) {
			t.Fatalf("want ErrCanceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop download")
	}
	task.releaseSem()
}

func TestCancelWhilePaused(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 8*1024*1024)
	srv := slowRangeServer(data)
	defer srv.Close()

	task := newTestTask(t)
	done := make(chan error, 1)
	go func() { done <- DownloadMedia(&bilibili.BiliClient{}, srv.URL+"/a", task, "video") }()
	time.Sleep(100 * time.Millisecond)
	task.SetPaused(true)
	time.Sleep(100 * time.Millisecond)
	task.Cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCanceled) {
			t.Fatalf("want ErrCanceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel while paused hung")
	}
	task.releaseSem()
}
