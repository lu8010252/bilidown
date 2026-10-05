import van, { State } from 'vanjs-core'
import { Route, goto, now } from 'vanjs-router'
import { checkLogin, GLOBAL_HAS_LOGIN, GLOBAL_HIDE_PAGE, ResJSON, VanComponent } from '../mixin'
import { deleteTask, fetchFileUrl, getActiveTask, getTaskList } from './data'
import { TaskInDB, TaskStatus } from '../work/type'
import { LoadingBox } from '../view'

const { div, span } = van.tags

const { svg, path } = van.tags('http://www.w3.org/2000/svg')

export class TaskRoute implements VanComponent {
    element: HTMLElement
    loading = van.state(false)

    taskList: State<(TaskInDB & {
        /** 音频下载进度百分比 */
        audioProgress: State<number>
        /** 视频下载进度百分比 */
        videoProgress: State<number>
        /** 合并进度百分比 */
        mergeProgress: State<number>
        /** 任务状态 */
        statusState: State<TaskStatus>
        /** 是否正在传输到本机 */
        transferring: State<boolean>
        /** 服务器上的文件是否已清理 */
        goneState: State<boolean>
        /** 是否正在删除 */
        deleting: State<boolean>
    })[]> = van.state([])

    constructor() {

        this.element = this.Root()
    }

    Root() {
        const _that = this
        return Route({
            rule: 'task',
            Loader() {
                return div(
                    () => _that.loading.val ? LoadingBox() : '',
                    () => div({ class: 'list-group', hidden: _that.loading.val },
                        _that.taskList.val.map(task => {
                            const ext = task.downloadType === 'audio' ? '.m4a' : '.mp4'
                            const filename = `${task.title} ${btoa(task.id.toString()).replace(/=/g, '')}${ext}`
                            return div({
                                class: () => `list-group-item p-0 hstack user-select-none ${task.statusState.val != 'done' && task.statusState.val != 'error' || task.transferring.val ? 'disabled' : ''}`,
                                hidden: task.deleting,
                            },
                                div({ class: 'vstack gap-2 py-2 px-3' },
                                    div({
                                        class: () => `
                                        ${task.statusState.val == 'error' ? 'text-danger' : ''}
                                        ${task.statusState.val == 'waiting' || task.statusState.val == 'running'
                                                ? 'text-primary' : ''}`
                                    },
                                        () => {
                                            if (task.transferring.val) return '正在传输到本机...'
                                            return div(
                                                span({
                                                    class: `me-2 badge ${task.downloadType === 'audio' ? 'bg-success' : 'bg-primary'}`,
                                                    title: task.downloadType === 'audio' ? '音频' : '视频'
                                                }, task.downloadType === 'audio' ? 'A' : 'V'),
                                                span({}, filename),
                                            )
                                        }),
                                    div({ class: 'text-secondary small' },
                                        () => {
                                            if (task.statusState.val == 'waiting') return '等待下载'
                                            if (task.statusState.val == 'error') return '下载失败'
                                            if (task.statusState.val == 'done') {
                                                if (task.goneState.val) return '已下载到本机，服务器文件已清理'
                                                return '文件在服务器上，点右侧 ↓ 下载到本机（传完后自动删除服务器文件）'
                                            }
                                            if (task.videoProgress.val == 0) {
                                                return `正在下载音频 (${(task.audioProgress.val * 100).toFixed(2)}%)`
                                            } else if (task.mergeProgress.val == 0) {
                                                return `正在下载视频 (${(task.videoProgress.val * 100).toFixed(2)}%)`
                                            } else if (task.statusState.val == 'running') {
                                                return `正在合并音视频 (${(task.mergeProgress.val * 100).toFixed(2)}%)`
                                            } else {
                                                return task.folder
                                            }
                                        }
                                    ),
                                    div({
                                        class: `progress`,
                                        style: `height: 5px`,
                                        hidden: () => task.statusState.val == 'done' || task.statusState.val == 'error'
                                    },
                                        div({
                                            class: () => `progress-bar progress-bar-striped progress-bar-animated bg-${(() => {
                                                if (task.videoProgress.val == 0) return 'primary'
                                                if (task.mergeProgress.val == 0) return 'success'
                                                else return 'info'
                                            })()}`,
                                            style: () => {
                                                let width = 0
                                                if (task.videoProgress.val == 0) width = task.audioProgress.val * 100
                                                else if (task.mergeProgress.val == 0) width = task.videoProgress.val * 100
                                                else width = task.mergeProgress.val * 100
                                                return `width: ${width}%`
                                            }
                                        }),
                                    )
                                ),
                                div({
                                    class: 'me-4',
                                    hidden: () => task.statusState.val != 'done'
                                        || task.goneState.val  // 服务器上已没有文件
                                        || task.transferring.val  // 正在传输时不重复触发
                                        || task.deleting.val
                                },
                                    div({
                                        class: 'hover-btn', title: '下载到本机（传完后删除服务器文件）',
                                        onclick() {
                                            task.transferring.val = true
                                            const link = document.createElement('a')
                                            link.href = fetchFileUrl(task.id)
                                            link.download = ''
                                            document.body.appendChild(link)
                                            link.click()
                                            link.remove()
                                            _that.waitFileGone(task)
                                        }
                                    },
                                        _that.DownloadSVG()
                                    )
                                ),
                                div({
                                    class: 'me-4',
                                    hidden: task.statusState.val != 'done'
                                        && task.statusState.val != 'error'
                                        || task.transferring.val  // 正在打开文件位置时，不应该显示删除按钮
                                        || task.deleting.val  // 正在删除时，不应该显示删除按钮
                                },
                                    div({
                                        class: 'hover-btn', title: '删除视频',
                                        onclick() {
                                            task.deleting.val = true
                                            deleteTask(task.id).then(() => {
                                                _that.taskList.val = _that.taskList.val.filter(taskInDB => taskInDB.id != task.id)
                                            }).catch(error => {
                                                alert(error.message)
                                            })
                                        }
                                    },
                                        _that.DeleteSVG()
                                    )
                                ),
                            )
                        })
                    )
                )
            },
            async onFirst() {
                if (!await checkLogin()) return
            },
            async onLoad() {
                if (!GLOBAL_HAS_LOGIN.val) return goto('login')
                _that.loading.val = true

                getTaskList(0, 360).then(taskList => {
                    if (!taskList) return
                    _that.taskList.val = taskList.map(task => ({
                        ...task,
                        audioProgress: van.state(1),
                        videoProgress: van.state(1),
                        mergeProgress: van.state(1),
                        statusState: van.state(task.status),
                        transferring: van.state(false),
                        goneState: van.state(!!task.fileGone),
                        deleting: van.state(false)
                    }))

                    const refresh = async () => {
                        const activeTaskList = await getActiveTask()
                        if (!activeTaskList) return false
                        setTimeout(() => {
                            _that.loading.val = false
                        }, 200)

                        _that.taskList.val.forEach(taskInDB => {
                            activeTaskList.forEach(task => {
                                if (taskInDB.id == task.id) {
                                    taskInDB.audioProgress.val = task.audioProgress
                                    taskInDB.videoProgress.val = task.videoProgress
                                    taskInDB.mergeProgress.val = task.mergeProgress
                                    taskInDB.statusState.val = task.status
                                }
                            })
                        })
                        if (activeTaskList.filter(task => task.status == 'running').length == 0) {
                            clearInterval(timer)
                            clearInterval(helper)
                        }
                        return true
                    }

                    refresh()

                    let timer = setInterval(() => {
                        refresh()
                    }, 1000)
                    let helper = setInterval(() => {
                        if (now.val.split('/')[0] != 'task') {
                            clearInterval(helper)
                            clearInterval(timer)
                        }
                    })
                })
            },
        })
    }

    DeleteSVG() {
        return svg({ style: `width: 1em; height: 1em`, fill: "currentColor", class: "bi bi-trash3", viewBox: "0 0 16 16" },
            path({ "d": "M6.5 1h3a.5.5 0 0 1 .5.5v1H6v-1a.5.5 0 0 1 .5-.5M11 2.5v-1A1.5 1.5 0 0 0 9.5 0h-3A1.5 1.5 0 0 0 5 1.5v1H1.5a.5.5 0 0 0 0 1h.538l.853 10.66A2 2 0 0 0 4.885 16h6.23a2 2 0 0 0 1.994-1.84l.853-10.66h.538a.5.5 0 0 0 0-1zm1.958 1-.846 10.58a1 1 0 0 1-.997.92h-6.23a1 1 0 0 1-.997-.92L3.042 3.5zm-7.487 1a.5.5 0 0 1 .528.47l.5 8.5a.5.5 0 0 1-.998.06L5 5.03a.5.5 0 0 1 .47-.53Zm5.058 0a.5.5 0 0 1 .47.53l-.5 8.5a.5.5 0 1 1-.998-.06l.5-8.5a.5.5 0 0 1 .528-.47M8 4.5a.5.5 0 0 1 .5.5v8.5a.5.5 0 0 1-1 0V5a.5.5 0 0 1 .5-.5" }),
        )
    }

    DownloadSVG() {
        return svg({ style: `width: 1em; height: 1em`, fill: "currentColor", class: "bi bi-download", viewBox: "0 0 16 16" },
            path({ "d": "M.5 9.9a.5.5 0 0 1 .5.5v2.5a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-2.5a.5.5 0 0 1 1 0v2.5a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2v-2.5a.5.5 0 0 1 .5-.5" }),
            path({ "d": "M7.646 11.854a.5.5 0 0 0 .708 0l3-3a.5.5 0 0 0-.708-.708L8.5 10.293V1.5a.5.5 0 0 0-1 0v8.793L5.354 8.146a.5.5 0 1 0-.708.708z" }),
        )
    }

    /**
     * 浏览器的文件下载由浏览器自己接管，页面无法得知何时完成，
     * 所以每 3 秒向服务器确认一次文件是否已被清理（服务器在完整传输后才会删除）。
     * 最多等待 2 小时；传输中断时文件会保留，到时恢复按钮即可重新下载。
     */
    waitFileGone(task: { id: number, transferring: State<boolean>, goneState: State<boolean> }) {
        const deadline = Date.now() + 2 * 3600 * 1000
        const timer = setInterval(async () => {
            try {
                const list = await getTaskList(0, 360)
                const item = list?.find(t => t.id == task.id)
                if (item?.fileGone) {
                    task.goneState.val = true
                    task.transferring.val = false
                    clearInterval(timer)
                    return
                }
            } catch (_) { /* 网络抖动时继续等待 */ }
            if (Date.now() > deadline || now.val.split('/')[0] != 'task') {
                task.transferring.val = false
                clearInterval(timer)
            }
        }, 3000)
    }
}

export default () => new TaskRoute().element