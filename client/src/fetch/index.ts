import van from 'vanjs-core'
import { ResJSON } from '../mixin'
import { ActiveTask, fetchFileUrl, getActiveTaskQuiet, getTaskListQuiet } from '../task/data'

/**
 * “取回”= 把服务器上已下载好的文件传到当前浏览器所在的电脑，传完后服务器删除该文件。
 * 这里的管理器独立于页面：自动取回、批量取回的队列在切换页面时也会继续执行。
 */
export type FetchState =
    | { kind: 'queued' }
    | { kind: 'fetching' }
    | { kind: 'done' }
    | { kind: 'failed', message: string }

const AUTO_KEY = 'bilidown.autoFetch'
/** 同时进行的取回数。浏览器对“连续多个下载”有限制，2 路比较稳。 */
const CONCURRENCY = 2

const loadAuto = () => {
    try {
        return localStorage.getItem(AUTO_KEY) !== '0'
    } catch {
        return true
    }
}

/** 是否在下载完成后自动取回（默认开启，保存在当前浏览器） */
export const autoFetch = van.state(loadAuto())

export const setAutoFetch = (on: boolean) => {
    autoFetch.val = on
    try {
        localStorage.setItem(AUTO_KEY, on ? '1' : '0')
    } catch { /* 隐私模式下仍在本次会话内生效 */ }
}

/** 本机模式（程序就运行在你自己的电脑上）：不需要取回，文件已经在本机 */
export const localMode = van.state(false)

export const fetchStates = van.state<Record<number, FetchState>>({})

const setState = (id: number, state: FetchState | null) => {
    const next = { ...fetchStates.val }
    if (state) next[id] = state
    else delete next[id]
    fetchStates.val = next
}

const queue: number[] = []
let running = 0

/** 让浏览器开始下载（服务端随后在完整传完后删除文件） */
const triggerDownload = (id: number) => {
    const link = document.createElement('a')
    link.href = fetchFileUrl(id)
    link.download = ''
    document.body.appendChild(link)
    link.click()
    link.remove()
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

/** 取回一个文件：触发下载，并向服务器确认是否完整传完 */
const fetchOne = async (id: number) => {
    setState(id, { kind: 'fetching' })
    triggerDownload(id)
    const startedAt = Date.now()
    let sawFetching = false
    while (true) {
        await sleep(2000)
        try {
            const item = (await getTaskListQuiet()).find(t => t.id == id)
            if (!item) return setState(id, null) // 任务已被删除
            if (item.fileGone) return setState(id, { kind: 'done' })
            if (item.fetching) sawFetching = true
            else if (sawFetching) {
                return setState(id, { kind: 'failed', message: '传输中断，服务器文件已保留，可重试' })
            } else if (Date.now() - startedAt > 20000) {
                return setState(id, {
                    kind: 'failed',
                    message: '浏览器没有开始下载。请在地址栏右侧允许本站“下载多个文件”，然后重试',
                })
            }
        } catch { /* 网络抖动：继续等 */ }
        if (Date.now() - startedAt > 2 * 3600 * 1000) {
            return setState(id, { kind: 'failed', message: '等待超时，可重试' })
        }
    }
}

const pump = () => {
    while (running < CONCURRENCY && queue.length) {
        const id = queue.shift()!
        running++
        fetchOne(id).finally(() => {
            running--
            pump()
        })
    }
}

/** 加入取回队列；已在队列或正在取回的会被跳过 */
export const enqueueFetch = (ids: number[]) => {
    for (const id of ids) {
        const state = fetchStates.val[id]
        if (state && (state.kind === 'queued' || state.kind === 'fetching')) continue
        setState(id, { kind: 'queued' })
        queue.push(id)
    }
    pump()
}

/** 自动取回：监视任务状态，从“未完成”变成“已完成”的任务会自动入队 */
const lastStatus = new Map<number, string>()
let firstPoll = true

const watchActive = async () => {
    if (localMode.val) return
    let list: ActiveTask[]
    try {
        list = await getActiveTaskQuiet()
    } catch {
        return
    }
    const finished: number[] = []
    for (const t of list) {
        const before = lastStatus.get(t.id)
        // 第一次轮询前就已经完成的任务不自动取回（可用“取回全部已完成”处理）
        if (!firstPoll && t.status === 'done' && before !== 'done') finished.push(t.id)
        lastStatus.set(t.id, t.status)
    }
    firstPoll = false
    if (autoFetch.val && finished.length) enqueueFetch(finished.reverse())
}

/** 应用启动时调用一次 */
export const initFetchManager = async () => {
    try {
        const res = await fetch('/api/getMode').then(res => res.json()) as ResJSON<{ local: boolean }>
        localMode.val = !!res.data?.local
    } catch { /* 取不到就按服务器模式处理 */ }
    watchActive()
    setInterval(watchActive, 3000)
}
