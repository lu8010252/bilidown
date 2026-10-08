package util

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"unicode/utf8"

	"bilidown/common"
)

func CheckBvidFormat(bvid string) bool {
	return regexp.MustCompile("^BV1[a-zA-Z0-9]+").MatchString(bvid)
}

// GetDefaultDownloadFolder 获取默认下载路径
func GetDefaultDownloadFolder() (string, error) {
	return filepath.Abs("./download")
}

func IsNumber(str string) bool {
	_, err := strconv.Atoi(str)
	return err == nil
}

// IsValidURL 判断字符串是否为合法的URL
func IsValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
}

// IsValidFormatCode 判断格式码是否合法
func IsValidFormatCode(format common.MediaFormat) bool {
	allowed := []common.MediaFormat{6, 16, 32, 64, 74, 80, 112, 116, 120, 125, 126, 127}
	for _, v := range allowed {
		if v == format {
			return true
		}
	}
	return false
}

// FilterFileName 过滤字符串中的特殊字符，使其允许作为文件名。
func FilterFileName(fileName string) string {
	return regexp.MustCompile(`[\\/:*?"<>|\n]`).ReplaceAllString(fileName, "")
}

// TruncateBytes 把字符串截断到不超过 max 字节，且不会切断一个汉字（UTF-8 字符）。
func TruncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// probeFFmpeg 试运行 `path -version`，能运行说明可用
func probeFFmpeg(path string) bool {
	cmd := exec.Command(path, "-version")
	HideWindow(cmd)
	return cmd.Run() == nil
}

// EmbeddedFFmpegPath 是内置 ffmpeg 释放后的路径（仅内置构建会设置），其他位置都找不到时才使用。
var EmbeddedFFmpegPath string

// GetFFmpegPath 获取可用的 FFmpeg 执行路径。
func GetFFmpegPath() (string, error) {
	if probeFFmpeg("ffmpeg") {
		return "ffmpeg", nil
	}
	if probeFFmpeg("bin/ffmpeg") {
		return "bin/ffmpeg", nil
	}
	// 程序所在目录下的 ffmpeg / bin/ffmpeg（Windows 上 Go 会自动补 .exe）
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, c := range []string{filepath.Join(dir, "ffmpeg"), filepath.Join(dir, "bin", "ffmpeg")} {
			if probeFFmpeg(c) {
				return c, nil
			}
		}
	}
	if EmbeddedFFmpegPath != "" && probeFFmpeg(EmbeddedFFmpegPath) {
		return EmbeddedFFmpegPath, nil
	}
	return "", errors.New("ffmpeg not found")
}

// GetRedirectedLocation 获取响应头中的 Location，不会自动跟随重定向。
func GetRedirectedLocation(url string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	if locationURL, err := response.Location(); err != nil {
		return "", err
	} else {
		return locationURL.String(), nil
	}
}

func MD5Hash(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	hash := hasher.Sum(nil)
	hashString := hex.EncodeToString(hash)
	return hashString
}
