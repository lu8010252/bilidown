import { ResJSON } from "../mixin"
import { TaskInDB, TaskStatus, VideoFormat } from "../work/type"

let getActiveTaskController: AbortController | undefined

export const getActiveTask = async (): Promise<ActiveTask[] | null> => {
    getActiveTaskController?.abort()
    getActiveTaskController = new AbortController()
    try {
        const res = await fetch('/api/getActiveTask', {
            signal: getActiveTaskController.signal
        }).then(res => res.json()) as ResJSON<ActiveTask[]>
        if (!res.success) throw new Error(res.message)
        return res.data
    } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') return null
        throw error
    }
}

/** 不会中止其他轮询的版本（取回管理器专用） */
export const getActiveTaskQuiet = async (): Promise<ActiveTask[]> => {
    const res = await fetch('/api/getActiveTask').then(res => res.json()) as ResJSON<ActiveTask[]>
    if (!res.success) throw new Error(res.message)
    return res.data
}

let getTaskListController: AbortController | undefined

export const getTaskList = async (page: number, pageSize: number): Promise<TaskInDB[] | null> => {
    getTaskListController?.abort()
    getTaskListController = new AbortController()
    try {
        const res = await fetch(`/api/getTaskList?page=${page}&pageSize=${pageSize}`, {
            signal: getTaskListController.signal
        }).then(res => res.json()) as ResJSON<TaskInDB[]>
        if (!res.success) throw new Error(res.message)
        return res.data
    } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') return null
        throw error
    }
}

/** 不与页面上其他请求互相取消的任务列表查询（取回管理器专用） */
export const getTaskListQuiet = async (): Promise<TaskInDB[]> => {
    const res = await fetch('/api/getTaskList?page=0&pageSize=360').then(res => res.json()) as ResJSON<TaskInDB[]>
    if (!res.success) throw new Error(res.message)
    return res.data
}

const post = async (action: string, id: number) => {
    const res = await fetch(`/api/${action}?id=${id}`).then(res => res.json()) as ResJSON
    if (!res.success) throw new Error(res.message)
}

export const pauseTask = (id: number) => post('pauseTask', id)
export const resumeTask = (id: number) => post('resumeTask', id)
export const cancelTask = (id: number) => post('cancelTask', id)

/** 已完成任务的“下载到本机”地址：浏览器完整收到文件后，服务器会删除该文件 */
export const fetchFileUrl = (id: number) => `/api/fetchFile?id=${id}`

/** 用于刷新任务实时进度 */
export type ActiveTask = {
    bvid: string
    cid: number
    /** 分辨率代码 */
    format: VideoFormat
    /** 视频标题 */
    title: string
    /** 视频发布者 */
    owner: string
    /** 视频封面 */
    cover: string
    /** 任务进度 */
    status: TaskStatus
    /** 文件保存到的目录 */
    folder: string
    /** 任务 ID */
    id: number
    /** 音频文件下载进度 */
    audioProgress: number
    /** 视频文件下载进度 */
    videoProgress: number
    /** 音视频合并进度 */
    mergeProgress: number
    /** 视频时长，秒 */
    duration: number
    /** 用户是否已暂停 */
    paused: boolean
}

export const deleteTask = async (id: number) => {
    const res = await fetch(`/api/deleteTask?id=${id}`).then(res => res.json()) as ResJSON
    if (!res.success) throw new Error(res.message)
}
/** 批量清理已结束（完成/失败）的任务。records：只清记录；all：连文件一起删 */
export const clearTasks = async (mode: 'records' | 'all'): Promise<number> => {
    const res = await fetch(`/api/clearTasks?mode=${mode}`).then(res => res.json()) as ResJSON<number>
    if (!res.success) throw new Error(res.message)
    return res.data
}
