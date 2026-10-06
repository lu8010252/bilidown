@echo off
chcp 65001 >nul
rem 在 Windows 上构建本机版。需要已安装 Node.js(含 corepack/pnpm) 和 Go 1.23+。
rem 用法：在项目根目录运行 windows\build-windows.bat ，产物在 dist-windows 目录。
setlocal
cd /d "%~dp0\.."
set GOPROXY=https://goproxy.cn,direct
set CGO_ENABLED=0

echo [1/3] 构建前端...
pushd client
call corepack enable
call pnpm config set registry https://registry.npmmirror.com
call pnpm install --frozen-lockfile || goto :fail
call pnpm build || goto :fail
popd

echo [2/3] 构建后端...
pushd server
go mod tidy || goto :fail
go build -trimpath -ldflags="-s -w -H=windowsgui" -o ..\dist-windows\bilidown.exe . || goto :fail
popd

echo [3/3] 整理文件...
copy /Y windows\README.txt dist-windows\README.txt >nul
echo.
echo 完成：dist-windows\bilidown.exe
echo 请把 ffmpeg.exe 放到 dist-windows 目录（或其 bin 子目录）后再运行。
exit /b 0

:fail
echo 构建失败，请查看上面的错误信息。
exit /b 1
