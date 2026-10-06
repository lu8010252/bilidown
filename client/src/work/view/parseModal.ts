import van, { State } from 'vanjs-core'
import { VanComponent, formatSeconds } from '../../mixin'
import { PageInParseResult, PlayInfo, VideoFormat } from '../type'
import { WorkRoute } from '..'
import { createTask, getPlayInfo } from '../data'
import PQueue from 'p-queue'

const { a, button, div, input, select, option, label, span } = van.tags

type Option = {
    workRoute: WorkRoute
}

const videoFormatMap: Record<VideoFormat, string> = {
    127: "超高清 8K",
    126: "杜比视界",
    125: "真彩 HDR",
    120: "超清 4K",
    116: "高清 1080P60",
    112: "高清 1080P+",
    80: "高清 1080P",
    74: "高清 720P60",
    64: "高清 720P",
    32: "清晰 480P",
    16: "流畅 360P",
    6: "极速 240P",
}

const codecMap: Record<12 | 7 | 13, string> = {
    12: "HEVC (hev1)",
    7: "AVC (avc1)",
    13: "AV1 (av01)",
}

/** 文件命名方式：parse 解析时的名称；full 完整信息；custom 自定义模板 */
type NamingMode = 'parse' | 'full' | 'custom'

const NAMING_KEY = 'bilidown.naming'
const DEFAULT_TEMPLATE = '{序号}. {分集名}'
/** 自定义模板里可用的占位符 */
const PLACEHOLDERS = ['序号', '分集名', '视频标题', 'UP主', '清晰度', '时长', 'BV号'] as const

const loadNaming = (): { mode: NamingMode, template: string } => {
    try {
        const data = JSON.parse(localStorage.getItem(NAMING_KEY) || '{}')
        return {
            mode: ['parse', 'full', 'custom'].includes(data.mode) ? data.mode : 'parse',
            template: typeof data.template === 'string' && data.template ? data.template : DEFAULT_TEMPLATE,
        }
    } catch {
        return { mode: 'parse', template: DEFAULT_TEMPLATE }
    }
}

const saveNaming = (mode: NamingMode, template: string) => {
    try {
        localStorage.setItem(NAMING_KEY, JSON.stringify({ mode, template }))
    } catch { /* 存储不可用时，本次会话内仍然生效 */ }
}

export class ParseModalComp implements VanComponent {
    element: HTMLElement

    totalCount: State<number>
    finishCount = van.state(0)

    abortControllers: AbortController[] = []

    currentController?: AbortController

    allPlayInfo: State<{
        page: PageInParseResult
        info: PlayInfo | null
        selected: State<boolean>
        formatIndex: State<number>
    }[]> = van.state([])

    /** 该属性用于在点击“开始下载”按钮后使按钮变为禁用状态，防止多次点击 */
    downloadBtnDisabled = van.state(false)

    /** 下载类型：audio 仅音频，video 仅视频，merge 音视频合并 */
    downloadType = van.state<'audio' | 'video' | 'merge'>('merge')

    /** 优先视频编码格式：12 hev1, 7 avc1, 13 av01 */
    preferredCodec = van.state<12 | 7 | 13>(12)
    /** 是否优先使用HiRes（flac）音频 */
    preferHiResAudio = van.state(true)

    errorList: State<string[]> = van.state([])

    /** 文件命名方式，选择会记在本浏览器里 */
    namingMode = van.state<NamingMode>(loadNaming().mode)
    /** 自定义命名模板，如 `{序号}. {分集名} [{清晰度}]` */
    namingTemplate = van.state(loadNaming().template)

    constructor(public option: Option) {
        van.derive(() => saveNaming(this.namingMode.val, this.namingTemplate.val))
        this.totalCount = van.derive(() => option.workRoute.selectedPages.val.length)
        const allFinish = van.derive(() => this.totalCount.val == this.finishCount.val)
        this.element = div({ class: `modal fade`, tabIndex: -1 },
            div({ class: () => `modal-dialog modal-xl modal-fullscreen-xl-down ${(this.totalCount.val + this.errorList.val.length) < 10 ? '' : 'modal-dialog-scrollable'}` },
                div({ class: `modal-content` },
                    div({ class: `modal-header` },
                        div({ class: `h5 modal-title` }, () => allFinish.val ? '批量下载' : '批量解析'),
                        button({ class: `btn-close`, 'data-bs-dismiss': `modal` })
                    ),
                    div({ class: `modal-body vstack gap-3`, tabIndex: -1, style: 'outline: none;' },
                        this.ParseProgress(),
                        div({ class: 'vstack gap-2', hidden: () => this.errorList.val.length == 0 || !allFinish.val },
                            div({ class: 'text-danger' }, () => `以下 ${this.errorList.val.length} 个视频解析失败`),
                            () => div({ class: 'list-group' },
                                this.errorList.val.map(error => div({ class: 'list-group-item disabled' }, error))
                            )
                        ),
                        this.ListGroup()
                    ),
                    this.ModalFooter()
                )
            )
        )

        this.element.addEventListener('hidden.bs.modal', () => {
            this.allPlayInfo.val = []
            this.finishCount.val = this.totalCount.val
            for (const controller of this.abortControllers) {
                controller.abort()
            }
            this.abortControllers = []
        })

        this.element.addEventListener('show.bs.modal', () => {
            this.start()
        })
    }

    /** 开始解析 */
    async start() {
        this.finishCount.val = 0
        this.errorList.val = []
        const queue = new PQueue({ concurrency: 10 })
        for (const page of this.option.workRoute.selectedPages.val) {
            queue.add(async () => {
                if (this.totalCount.val == this.finishCount.val) return
                const controller = new AbortController()
                this.abortControllers.push(controller)
                const playInfo = await getPlayInfo(page.bvid, page.cid, controller)
                playInfo.accept_quality = [...new Set(playInfo.dash.video.map(video => video.id))].sort((a, b) => b - a)
                this.allPlayInfo.val = this.allPlayInfo.val.concat({
                    page,
                    info: playInfo,
                    selected: van.state(true),
                    formatIndex: van.state(0),
                })
                this.finishCount.val++
            }).catch(() => {
                this.finishCount.val++
                const badgeNotNum = !page.badge.match(/^\d+$/)
                this.errorList.val = this.errorList.val.concat(`${page.part}${badgeNotNum ? ` - ${page.badge}` : ''}`)
            })
        }
        await queue.onIdle()
    }

    /** 视频发布者（多人合作时取第一位） */
    private ownerName() {
        const data = this.option.workRoute.videoInfoCardData.val
        return data.staff.length > 0 ? data.staff[0].split('[')[0].trim() : data.owner.name.trim()
    }

    /** 按当前选择的命名方式生成文件名（不含扩展名） */
    makeTitle(info: ParseModalComp['allPlayInfo']['val'][number]): string {
        const clean = (text: string) => text.replace(/\s+/g, ' ').trim()
        const workRoute = this.option.workRoute
        const data = workRoute.videoInfoCardData.val
        const cardTitle = data.title.trim()
        const badge = info.page.badge.trim()
        const part = info.page.part.trim()
        const badgeIsNum = /^\d+$/.test(badge)
        const formatName = videoFormatMap[info.info!.accept_quality[info.formatIndex.val]]
        const duration = formatSeconds(info.info!.dash.duration)

        // 与解析列表里每一行显示的名字一致：数字序号 + “. ” + 分集名
        const parseName = clean((badgeIsNum && badge ? `${badge}. ` : '') + (part || cardTitle))

        if (this.namingMode.val === 'parse') return parseName || cardTitle

        if (this.namingMode.val === 'custom') {
            const values: Record<typeof PLACEHOLDERS[number], string> = {
                '序号': badge, '分集名': part, '视频标题': cardTitle, 'UP主': this.ownerName(),
                '清晰度': formatName, '时长': duration, 'BV号': info.page.bvid,
            }
            const name = clean(this.namingTemplate.val.replace(
                /\{(序号|分集名|视频标题|UP主|清晰度|时长|BV号)\}/g,
                (_, key: typeof PLACEHOLDERS[number]) => values[key]
            ))
            return name || parseName || cardTitle
        }

        // full：原版的完整格式（合集名、序号、UP 主、清晰度、时长都带上）
        const isVideoMode = workRoute.videoInfoCardMode.val == 'video'
        const pagesLength = data.pages.length
        return (!badgeIsNum
            ? [part, `[${badge}]`, `[${cardTitle}]`, `[${formatName}]`, `[${duration}]`]
            : [
                pagesLength == 1 ? workRoute.allSection.val[workRoute.sectionTabsActiveIndex.val].title : `[${cardTitle}]`,
                workRoute.sectionPages.val.length == 1 ? '' : `[${badge}]`,
                part,
                isVideoMode ? `[${this.ownerName()}]` : '',
                `[${formatName}]`,
                `[${duration}]`,
            ]).filter(p => p).join(' ')
    }

    download() {
        const selectedPlayInfos = this.allPlayInfo.val.filter(info => info.selected.val)
        const workRoute = this.option.workRoute
        this.downloadBtnDisabled.val = true
        // 需要传递给服务器，需要创建下载任务的数据列表
        createTask(selectedPlayInfos.map(info => {
            const owner = this.ownerName()
            const activeVideoInfo = getActiveFormatVideo(info.info!, info.info!.accept_quality[info.formatIndex.val], this.preferredCodec.val)

            return ({
                bvid: info.page.bvid,
                cid: info.page.cid,
                cover: workRoute.videoInfoCardData.val.cover,
                title: this.makeTitle(info),
                format: info.info!.accept_quality[info.formatIndex.val],
                owner,
                audio: getAudioURL(info.info!, this.preferHiResAudio.val),
                duration: info.info!.dash.duration,
                downloadType: this.downloadType.val,
                ...activeVideoInfo
            })
        })).then(() => {
            workRoute.parseModal.hide()
        }).catch(error => {
            alert(error.message)
        }).finally(() => {
            this.downloadBtnDisabled.val = false
        })
    }

    ParseProgress() {
        return div({ class: 'vstack gap-3', hidden: () => this.totalCount.val == this.finishCount.val },
            div({ class: 'text-center fs-5' }, () => `正在解析，剩余 ${this.totalCount.val - this.finishCount.val} 项`),
            div({ class: 'progress' }, div({
                class: 'progress-bar progress-bar-striped progress-bar-animated',
                style: () => `width: ${this.finishCount.val / this.totalCount.val * 100}%`
            },)),
        )
    }

    ListGroup() {
        return () => div({ class: 'list-group', hidden: () => this.totalCount.val != this.finishCount.val },
            this.allPlayInfo.val.filter(info => info.info)
                .sort((a, b) => a.page.page - b.page.page)
                .map(info => {
                    const badgeNotNum = !info.page.badge.match(/^\d+$/)

                    return div({
                        class: () => `list-group-item user-select-none py-0 ${info.info ? '' : 'disabled'}`,
                        role: 'button',
                        onclick(event) {
                            if ((event.target as HTMLElement).getAttribute('class')?.match(/dropdown-?/)) return
                            info.selected.val = !info.selected.val
                        }
                    },
                        div({ class: 'hstack gap-2' },
                            div({ class: 'hstack gap-3 flex-fill py-1' },
                                input({
                                    class: 'form-check-input', type: 'checkbox', checked: info.selected,
                                }),
                                div({},
                                    div((badgeNotNum ? '' : `${info.page.badge}. `) + info.page.part),
                                    badgeNotNum ? div({ class: info.page.part ? 'small text-secondary' : '' },
                                        info.page.badge) : ''
                                ),
                            ),
                            div({ class: 'dropdown' },
                                div({ class: 'dropdown-toggle py-2 text-primary', 'data-bs-toggle': 'dropdown' },
                                    () => videoFormatMap[info.info!.accept_quality[info.formatIndex.val]]
                                ),
                                () => {
                                    return div({ class: 'dropdown-menu shadow' },
                                        info.info!.accept_quality.map((formatID, index) => {
                                            return div({
                                                class: () => `dropdown-item ${info.formatIndex.val == index ? 'active' : ''}`,
                                                onclick() {
                                                    info.formatIndex.val = index
                                                }
                                            }, videoFormatMap[formatID as VideoFormat])
                                        })
                                    )
                                }
                            )
                        )
                    )
                })
        )
    }

    /** “文件命名”控件：预设 + 自定义模板 + 第一项的预览 */
    NamingControls() {
        const _that = this
        const preview = van.derive(() => {
            const first = _that.allPlayInfo.val.find(info => info.selected.val && info.info)
            return first ? _that.makeTitle(first) : ''
        })

        return div({ class: 'vstack gap-2 mt-2' },
            div({ class: 'hstack gap-2 flex-wrap' },
                span({ class: 'text-nowrap' }, '文件命名'),
                select({
                    class: 'form-select form-select-sm w-auto',
                    value: _that.namingMode,
                    oninput: (e) => _that.namingMode.val = (e.target as HTMLSelectElement).value as NamingMode
                },
                    option({ value: 'parse' }, '解析时的名称（默认）'),
                    option({ value: 'full' }, '完整信息（合集、UP主、清晰度、时长）'),
                    option({ value: 'custom' }, '自定义…')
                ),
            ),
            () => _that.namingMode.val == 'custom' ? div({ class: 'vstack gap-2' },
                input({
                    class: 'form-control form-control-sm',
                    placeholder: '例如：{序号}. {分集名} [{清晰度}]；只下载一个视频时，也可以直接输入想要的名字',
                    value: _that.namingTemplate,
                    oninput: (e) => _that.namingTemplate.val = (e.target as HTMLInputElement).value
                }),
                div({ class: 'hstack gap-1 flex-wrap' },
                    span({ class: 'small text-secondary text-nowrap' }, '点击插入：'),
                    PLACEHOLDERS.map(name => button({
                        type: 'button', class: 'btn btn-sm btn-outline-secondary py-0',
                        onclick: () => _that.namingTemplate.val += `{${name}}`
                    }, name))
                )
            ) : '',
            div({ class: 'small text-secondary text-break text-wrap', hidden: () => !preview.val },
                () => `示例：${preview.val}`)
        )
    }

    ModalFooter() {
        const _that = this

        const selectedCount = van.derive(() => this.allPlayInfo.val.filter(info => info.selected.val).length)
        const totalCount = van.derive(() => this.allPlayInfo.val.length)
        /** 解析完成列表全部选中 */
        const allSelected = van.derive(() => selectedCount.val == totalCount.val)
        /** 是否全部解析完成 */
        const allFinish = van.derive(() => this.totalCount.val == this.finishCount.val)

        return div({ class: `modal-footer` },
            div({ class: 'me-auto', hidden: () => !allFinish.val || totalCount.val == 0 },
                div({ class: 'hstack gap-3 text-nowrap' },
                    () => `已选择 (${selectedCount.val}/${totalCount.val}) 项`,
                    select({
                        class: 'form-select form-select-sm',
                        value: _that.downloadType,
                        oninput: (e) => _that.downloadType.val = (e.target as HTMLSelectElement).value as 'audio' | 'video' | 'merge'
                    },
                        option({ value: 'merge' }, '音视频合并'),
                        option({ value: 'audio' }, '仅音频'),
                        option({ value: 'video' }, '仅视频')
                    ),
                    select({
                        class: 'form-select form-select-sm',
                        value: _that.preferredCodec,
                        oninput: (e) => _that.preferredCodec.val = Number((e.target as HTMLSelectElement).value) as 12 | 7 | 13
                    },
                        option({ value: '12' }, 'HEVC (hev1)'),
                        option({ value: '7' }, 'AVC (avc1)'),
                        option({ value: '13' }, 'AV1 (av01)')
                    ),
                    div({ class: 'form-check form-check-inline' },
                        input({
                            type: 'checkbox',
                            class: 'form-check-input',
                            id: 'preferHiResAudio',
                            checked: _that.preferHiResAudio,
                            oninput: (e) => _that.preferHiResAudio.val = (e.target as HTMLInputElement).checked
                        }),
                        label({ class: 'form-check-label', for: 'preferHiResAudio' }, 'Hi-Res')
                    )
                ),
                _that.NamingControls(),
            ),
            button({
                class: `btn btn-secondary`,
                'data-bs-dismiss': `modal`,
                hidden: allFinish
            }, '取消解析'),
            button({
                class: 'btn btn-secondary', hidden: () => !allFinish.val || allSelected.val || totalCount.val == 0,
                onclick() {
                    _that.allPlayInfo.val.forEach(info => info.selected.val = true)
                }
            }, '全选'),
            button({
                class: 'btn btn-warning', hidden: () => !allFinish.val || !allSelected.val || totalCount.val == 0,
                onclick() {
                    _that.allPlayInfo.val.forEach(info => info.selected.val = false)
                }
            }, '全不选'),
            button({
                class: `btn btn-primary`, onclick() {
                    _that.download()
                },
                hidden: () => !allFinish.val,
                disabled: () => selectedCount.val <= 0 || _that.downloadBtnDisabled.val
            }, '开始下载'),
        )
    }
}

const getAudioURL = (playInfo: PlayInfo, preferHiRes: boolean = true): string => {
    if (preferHiRes && playInfo.dash.flac) {
        return playInfo.dash.flac.audio.baseUrl
    } else {
        return playInfo.dash.audio.sort((a, b) => b.id - a.id)[0].baseUrl
    }
}

const getActiveFormatVideo = (playInfo: PlayInfo, format: VideoFormat, preferredCodec: 12 | 7 | 13 = 12): { video: string, width: number, height: number } => {
    // 优先级顺序：用户首选编码格式，然后按默认优先级 12 > 7 > 13
    const codecOrder = [preferredCodec, 12, 7, 13].filter((value, index, self) => self.indexOf(value) === index)
    for (const code of codecOrder) {
        for (const item of playInfo.dash.video) {
            if (item.id == format && item.codecid == code) {
                return {
                    video: item.baseUrl,
                    width: item.width,
                    height: item.height
                }
            }
        }
    }
    throw new Error('未找到对应视频分辨率格式')
}
