import van from 'vanjs-core'

/**
 * 运行模式由后端决定：
 * - 无头模式（服务器 / Docker）：文件留在服务器上，任务页提供“取回到本机”
 * - 本机模式（Windows 托盘版）：文件就在本机，任务页提供“打开位置”
 */
export const headless = van.state(false)

/** 向后端查询运行模式；失败时按本机模式处理 */
export const loadMode = async () => {
    try {
        const res = await fetch('/api/mode').then(res => res.json())
        headless.val = !!res.data?.headless
    } catch {
        headless.val = false
    }
}
