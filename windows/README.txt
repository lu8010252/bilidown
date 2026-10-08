Bilidown Windows 版
====================

使用：
1. 从 GitHub Actions / Releases 下载的 exe 已内置 ffmpeg，可跳过这一步（首次运行会自动释放到 bin\ffmpeg.exe）；
   自己编译的话，把 ffmpeg.exe 放到 bilidown.exe 旁边（或旁边的 bin 文件夹）。
   如果 bilidown.exe 旁边已有 ffmpeg.exe，会优先用你自己的。
2. 双击 bilidown.exe。没有黑窗口，右下角系统托盘会出现图标，并自动用默认浏览器打开
   http://127.0.0.1:8098 。首次使用在页面里扫码登录 B 站。
3. 托盘图标右键菜单：打开主界面 / 打开下载目录 / Github 项目主页 / 退出应用。
   关闭浏览器标签页不会退出程序；要退出请用托盘菜单“退出应用”。

说明：
- 只需要 bilidown.exe 一个文件，页面、图标和 ffmpeg 都已打包在 exe 里。
- 只监听 127.0.0.1，局域网内其他设备访问不到。
- 视频默认保存在 exe 旁边的 download 文件夹（可在“设置中心”修改），
  任务列表里点“打开位置”会在资源管理器里选中文件。
- 数据（登录状态、任务记录）在 data.db，运行日志在 bilidown.log，都在 exe 旁边。
- 再次双击 bilidown.exe：如果已经在运行，只会重新打开浏览器页面。
- 找不到 ffmpeg 或端口被占用时会弹出提示框。换端口：在命令行里先执行
  set BILIDOWN_PORT=8099 再运行。
