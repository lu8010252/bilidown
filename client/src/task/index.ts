import van, { State } from 'vanjs-core'
import { Route, goto, now } from 'vanjs-router'
import { checkLogin, formatBytes, GLOBAL_HAS_LOGIN, VanComponent } from '../mixin'
import { cancelTask, deleteTask, getActiveTask, getTaskList, pauseTask, resumeTask, showFile } from './data'
import { TaskInDB, TaskStatus } from '../work/type'
import { LoadingBox } from '../view'
import { autoFetch, enqueueFetch, fetchStates, localMode, setAutoFetch } from '../fetch'

const { button, div, input, label, span } = van.tags

const { svg, path } = van.tags('http://www.w3.org/2000/svg')

type Row = TaskInDB & {
    /** 音频下载进度百分比 */
    audioProgress: State<number>
    /** 视频下载进度百分比 */
    videoProgress: State<number>
    /** 合并进度百分比 */
    mergeProgress: State<number>
    /** 任务状态 */
    statusState: State<TaskStatus>
    /** 是否已暂停 */
    paused: State<boolean>
    /** 服务器上的文件是否已清理 */
    goneState: State<boolean>
    /** 是否正在删除 */
    deleting: State<boolean>
}

export class TaskRoute implements VanComponent {
    element: HTMLElement
    loading = van.state(false)

    taskList: State<Row[]> = van.state([])

    /** 批量取回时勾选的任务 ID */
    selected = van.state<number[]>([])

    constructor() {

        this.element = this.Root()
    }

    /** 文件已经不在服务器上（已取回并清理） */
    isGone(task: Row) {
        return task.goneState.val || fetchStates.val[task.id]?.kind === 'done'
    }

    /** 可以取回：已完成、文件还在服务器上、且没有正在排队/传输 */
    isFetchable(task: Row) {
        if (task.statusState.val !== 'done' || this.isGone(task)) return false
        const kind = fetchStates.val[task.id]?.kind
        return kind !== 'queued' && kind !== 'fetching'
    }

    /** 下载中（含排队、暂停），且还没进入合并阶段，才能暂停/取消 */
    isControllable(task: Row) {
        return (task.statusState.val === 'running' || task.statusState.val === 'waiting') && task.mergeProgress.val === 0
    }

    Toolbar() {
        const _that = this
        const fetchable = () => _that.taskList.val.filter(task => _that.isFetchable(task))
        const selectedFetchable = () => fetchable().filter(task => _that.selected.val.includes(task.id))
        return div({
            class: 'app-panel p-2 px-3 hstack gap-3 flex-wrap',
            hidden: () => localMode.val,
        },
            div({ class: 'form-check form-switch mb-0' },
                input({
                    class: 'form-check-input', type: 'checkbox', role: 'switch', id: 'auto-fetch-switch',
                    checked: () => autoFetch.val,
                    onchange: (event: Event) => setAutoFetch((event.target as HTMLInputElement).checked),
                }),
                label({
                    class: 'form-check-label', for: 'auto-fetch-switch',
                    title: '开启后，每个下载完成的文件会自动传到本机，传完服务器自动删除',
                }, '下载完成后自动取回'),
            ),
            div({ class: 'form-check mb-0', hidden: () => fetchable().length === 0 },
                input({
                    class: 'form-check-input', type: 'checkbox', id: 'fetch-select-all',
                    checked: () => fetchable().length > 0 && selectedFetchable().length === fetchable().length,
                    onchange: (event: Event) => {
                        _that.selected.val = (event.target as HTMLInputElement).checked
                            ? fetchable().map(task => task.id) : []
                    },
                }),
                label({ class: 'form-check-label', for: 'fetch-select-all' }, '全选'),
            ),
            button({
                class: 'btn btn-sm btn-primary',
                disabled: () => selectedFetchable().length === 0,
                onclick() {
                    enqueueFetch(selectedFetchable().map(task => task.id))
                    _that.selected.val = []
                }
            }, () => `取回选中 (${selectedFetchable().length})`),
            button({
                class: 'btn btn-sm btn-outline-primary',
                disabled: () => fetchable().length === 0,
                onclick() {
                    enqueueFetch(fetchable().map(task => task.id))
                    _that.selected.val = []
                }
            }, () => `取回全部已完成 (${fetchable().length})`),
        )
    }

    Subtitle(task: Row) {
        const _that = this
        return () => {
            const status = task.statusState.val
            if (status === 'waiting') return task.paused.val ? '已暂停（排队中）' : '等待下载'
            if (status === 'error') return '下载失败'
            if (status === 'done') {
                if (localMode.val) return `已保存到 ${task.folder}`
                if (_that.isGone(task)) return '已取回到本机，服务器文件已清理'
                const state = fetchStates.val[task.id]
                if (state?.kind === 'queued') return '排队等待取回…'
                if (state?.kind === 'fetching') return '正在传输到本机…'
                if (state?.kind === 'failed') return `取回失败：${state.message}`
                const size = task.fileSize ? `（${formatBytes(task.fileSize)}）` : ''
                return `文件在服务器上${size}，点右侧 ↓ 取回到本机（传完后自动删除服务器文件）`
            }
            const prefix = task.paused.val ? '已暂停 ' : '正在'
            if (task.videoProgress.val == 0) {
                return `${prefix}下载音频 (${(task.audioProgress.val * 100).toFixed(2)}%)`
            } else if (task.mergeProgress.val == 0) {
                return `${prefix}下载视频 (${(task.videoProgress.val * 100).toFixed(2)}%)`
            } else {
                return `正在合并音视频 (${(task.mergeProgress.val * 100).toFixed(2)}%)`
            }
        }
    }

    Row(task: Row) {
        const _that = this
        const ext = task.downloadType === 'audio' ? '.m4a' : '.mp4'
        const filename = `${task.title}${ext}`
        const iconBtn = (title: string, hidden: () => boolean, icon: Element, onclick: () => void) => div({
            class: 'me-3', hidden,
        }, div({ class: 'hover-btn', title, onclick }, icon))

        return div({
            class: 'list-group-item p-0 hstack user-select-none',
            hidden: task.deleting,
        },
            div({ class: 'ps-3', hidden: () => localMode.val || !_that.isFetchable(task) },
                input({
                    class: 'form-check-input', type: 'checkbox', title: '选择，用于批量取回',
                    checked: () => _that.selected.val.includes(task.id),
                    onchange: (event: Event) => {
                        const on = (event.target as HTMLInputElement).checked
                        _that.selected.val = on
                            ? [..._that.selected.val, task.id]
                            : _that.selected.val.filter(id => id !== task.id)
                    },
                })
            ),
            div({ class: 'vstack gap-2 py-2 px-3' },
                div({
                    class: () => `
                    ${task.statusState.val == 'error' ? 'text-danger' : ''}
                    ${task.statusState.val == 'waiting' || task.statusState.val == 'running'
                            ? (task.paused.val ? 'text-warning' : 'text-primary') : ''}`
                },
                    span({
                        class: `me-2 badge ${task.downloadType === 'audio' ? 'bg-success' : 'bg-primary'}`,
                        title: task.downloadType === 'audio' ? '音频' : '视频'
                    }, task.downloadType === 'audio' ? 'A' : 'V'),
                    span({}, filename),
                ),
                div({ class: 'text-secondary small' }, _that.Subtitle(task)),
                div({
                    class: `progress`,
                    style: `height: 5px`,
                    hidden: () => task.statusState.val == 'done' || task.statusState.val == 'error'
                },
                    div({
                        class: () => `progress-bar ${task.paused.val ? 'bg-warning' : 'progress-bar-striped progress-bar-animated'} bg-${(() => {
                            if (task.paused.val) return 'warning'
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

            // 暂停 / 继续
            iconBtn('暂停', () => !_that.isControllable(task) || task.paused.val, _that.PauseSVG(), () => {
                pauseTask(task.id).then(() => { task.paused.val = true }).catch(error => alert(error.message))
            }),
            iconBtn('继续', () => !_that.isControllable(task) || !task.paused.val, _that.PlaySVG(), () => {
                resumeTask(task.id).then(() => { task.paused.val = false }).catch(error => alert(error.message))
            }),
            // 取消下载（会删除已下载的部分和任务记录）
            iconBtn('取消下载', () => !_that.isControllable(task), _that.CancelSVG(), () => {
                if (!confirm('取消这个下载？已下载的部分会被删除。')) return
                cancelTask(task.id).then(() => {
                    _that.taskList.val = _that.taskList.val.filter(t => t.id != task.id)
                }).catch(error => alert(error.message))
            }),

            // 服务器模式：取回到本机
            iconBtn('取回到本机（传完后删除服务器文件）',
                () => localMode.val || !_that.isFetchable(task) || task.deleting.val,
                _that.DownloadSVG(),
                () => enqueueFetch([task.id])),
            // 本机模式：在资源管理器中定位
            div({
                class: 'me-3', hidden: () => !localMode.val || task.statusState.val != 'done' || task.goneState.val,
            }, button({
                class: 'btn btn-sm btn-outline-secondary text-nowrap',
                onclick: () => showFile(task.id).catch(error => alert(error.message)),
            }, '打开位置')),

            iconBtn('删除视频',
                () => task.statusState.val != 'done' && task.statusState.val != 'error'
                    || fetchStates.val[task.id]?.kind === 'fetching'
                    || task.deleting.val,
                _that.DeleteSVG(),
                () => {
                    task.deleting.val = true
                    deleteTask(task.id).then(() => {
                        _that.taskList.val = _that.taskList.val.filter(taskInDB => taskInDB.id != task.id)
                    }).catch(error => {
                        task.deleting.val = false
                        alert(error.message)
                    })
                }),
        )
    }

    Root() {
        const _that = this
        return Route({
            rule: 'task',
            Loader() {
                return div({ class: 'vstack gap-3' },
                    _that.Toolbar(),
                    () => _that.loading.val ? LoadingBox() : '',
                    () => div({ class: 'list-group', hidden: _that.loading.val },
                        _that.taskList.val.map(task => _that.Row(task))
                    )
                )
            },
            async onFirst() {
                if (!await checkLogin()) return
            },
            async onLoad() {
                if (!GLOBAL_HAS_LOGIN.val) return goto('login')
                _that.loading.val = true
                _that.selected.val = []

                getTaskList(0, 360).then(taskList => {
                    if (!taskList) return
                    _that.taskList.val = taskList.map(task => ({
                        ...task,
                        audioProgress: van.state(1),
                        videoProgress: van.state(1),
                        mergeProgress: van.state(1),
                        statusState: van.state(task.status),
                        paused: van.state(false),
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
                                    taskInDB.paused.val = !!task.paused
                                    if (taskInDB.statusState.val != task.status) {
                                        taskInDB.statusState.val = task.status
                                        // 刚完成的任务：补充真实文件大小
                                        if (task.status == 'done') getTaskList(0, 360).then(list => {
                                            const item = list?.find(t => t.id == task.id)
                                            if (item) taskInDB.fileSize = item.fileSize
                                        }).catch(() => { })
                                    }
                                }
                            })
                        })
                        // 还有排队中或下载中的任务就继续刷新
                        if (activeTaskList.filter(task => task.status == 'running' || task.status == 'waiting').length == 0) {
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

    PauseSVG() {
        return svg({ style: `width: 1em; height: 1em`, fill: "currentColor", class: "bi bi-pause-fill", viewBox: "0 0 16 16" },
            path({ "d": "M5.5 3.5A1.5 1.5 0 0 1 7 5v6a1.5 1.5 0 0 1-3 0V5a1.5 1.5 0 0 1 1.5-1.5m5 0A1.5 1.5 0 0 1 12 5v6a1.5 1.5 0 0 1-3 0V5a1.5 1.5 0 0 1 1.5-1.5" }),
        )
    }

    PlaySVG() {
        return svg({ style: `width: 1em; height: 1em`, fill: "currentColor", class: "bi bi-play-fill", viewBox: "0 0 16 16" },
            path({ "d": "m11.596 8.697-6.363 3.692c-.54.313-1.233-.066-1.233-.697V4.308c0-.63.692-1.01 1.233-.696l6.363 3.692a.802.802 0 0 1 0 1.393" }),
        )
    }

    CancelSVG() {
        return svg({ style: `width: 1em; height: 1em`, fill: "currentColor", class: "bi bi-x-lg", viewBox: "0 0 16 16" },
            path({ "d": "M2.146 2.854a.5.5 0 1 1 .708-.708L8 7.293l5.146-5.147a.5.5 0 0 1 .708.708L8.707 8l5.147 5.146a.5.5 0 0 1-.708.708L8 8.707l-5.146 5.147a.5.5 0 0 1-.708-.708L7.293 8z" }),
        )
    }
}

export default () => new TaskRoute().element
