import van, { State } from 'vanjs-core'
import { VideoParseResult, VideoInfoCardMode, PageInParseResult, SectionItem } from '../type'
import { VanComponent } from '../../mixin'
import { WorkRoute } from '..'

const { button, div, span } = van.tags

class VideoItemListComp implements VanComponent {
    element: HTMLElement

    constructor(
        public workRoute: WorkRoute
    ) {
        const { videoInfoCardData: data } = workRoute

        this.element = div({
            hidden: () => false && data.val.pages.length <= 1,
            class: 'vstack gap-4'
        },
            div({ class: 'vstack gap-4' },
                div({ hidden: () => workRoute.allSection.val.length == 1 && workRoute.allSection.val[0].title == '正片' }, SectionTabs(this, workRoute.allSection)),
                ButtonGroup(workRoute),
                ListBox(workRoute.sectionPages),
            )
        )
    }
}

const SectionTabs = (parent: VideoItemListComp, allSection: State<SectionItem[]>) => {
    return () => div({ class: 'nav nav-underline' },
        allSection.val.map((item, index) => div({ class: 'nav-item', role: 'button' },
            div({
                tabIndex: 0,
                class: `nav-link ${parent.workRoute.sectionTabsActiveIndex.val == index ? 'active' : ''}`,
                onclick() {
                    parent.workRoute.sectionTabsActiveIndex.val = index
                },
                onkeyup(e) {
                    if (e.key == 'Enter') {
                        e.target.click()
                    }
                }
            }, () => item.title)
        ))
    )
}

const ButtonGroup = (workRoute: WorkRoute) => {
    const pages = workRoute.sectionPages
    const selectedCount = van.derive(() => pages.val.filter(page => page.selected.val).length)
    const totalCount = van.derive(() => pages.val.length)

    return div({ class: 'vstack gap-2' },
        div({ class: 'hstack gap-3 flex-wrap' },
            button({
                class: 'btn btn-secondary',
                onclick() {
                    pages.val.forEach(page => page.selected.val = selectedCount.val < totalCount.val)
                }
            }, () => `${selectedCount.val < totalCount.val ? '全选' : '取消全选'} (${selectedCount.val}/${totalCount.val})`),
            button({
                class: 'btn btn-outline-secondary',
                onclick() {
                    pages.val.forEach(page => page.selected.val = !page.selected.val)
                }
            }, '反选'),
            button({
                class: 'btn btn-primary',
                disabled: () => selectedCount.val <= 0,
                async onclick() {
                    workRoute.parseModal.show()
                }
            }, '解析选中项目')
        ),
        div({ class: 'small text-muted' }, '提示：按住 Shift 点击可连选一段；在列表中按住鼠标拖动可框选（拖到窗口边缘会自动滚动），按住 Ctrl 或 Shift 拖动可追加选择。')
    )
}

/** 框选用的半透明矩形，挂在 body 上，使用页面坐标 */
let selectionBox: HTMLDivElement | null = null
const getSelectionBox = () => {
    if (!selectionBox) {
        selectionBox = document.createElement('div')
        selectionBox.style.cssText = 'position:absolute;display:none;pointer-events:none;z-index:2000;'
            + 'border:1px solid var(--bs-primary,#0d6efd);background:rgba(13,110,253,.15);border-radius:2px;'
        document.body.appendChild(selectionBox)
    }
    return selectionBox
}

const ListBox = (pages: State<PageInParseResult[]>) => {
    // Shift 连选的起点；列表（切换分区）变化后重置
    let anchor = -1
    let anchorFor: PageInParseResult[] | null = null
    // 框选结束后会紧跟一个 click，需要吞掉
    let suppressClick = false

    const onCardClick = (list: PageInParseResult[], index: number, e: MouseEvent | KeyboardEvent) => {
        if (anchorFor !== list) {
            anchorFor = list
            anchor = -1
        }
        if (e.shiftKey && anchor >= 0 && anchor < list.length) {
            const value = list[anchor].selected.val
            const [from, to] = anchor < index ? [anchor, index] : [index, anchor]
            for (let i = from; i <= to; i++) list[i].selected.val = value
            // 保持起点不变，方便继续 Shift 点击调整范围
            return
        }
        list[index].selected.val = !list[index].selected.val
        anchor = index
    }

    const enableBoxSelect = (container: HTMLElement, list: PageInParseResult[]) => {
        container.addEventListener('click', e => {
            if (suppressClick) {
                e.stopPropagation()
                e.preventDefault()
                suppressClick = false
            }
        }, true)
        container.addEventListener('pointerdown', e => {
            if (e.pointerType !== 'mouse' || e.button !== 0) return
            const startX = e.pageX
            const startY = e.pageY
            const additive = e.ctrlKey || e.metaKey || e.shiftKey
            const base = list.map(page => page.selected.val)
            const box = getSelectionBox()
            let dragging = false
            let pointerX = e.clientX
            let pointerY = e.clientY
            let raf = 0

            const update = () => {
                const curX = pointerX + window.scrollX
                const curY = pointerY + window.scrollY
                const left = Math.min(startX, curX)
                const top = Math.min(startY, curY)
                const right = Math.max(startX, curX)
                const bottom = Math.max(startY, curY)
                box.style.left = `${left}px`
                box.style.top = `${top}px`
                box.style.width = `${right - left}px`
                box.style.height = `${bottom - top}px`
                const cards = container.querySelectorAll<HTMLElement>('[data-item-index]')
                cards.forEach(card => {
                    const index = Number(card.dataset.itemIndex)
                    const r = card.getBoundingClientRect()
                    const hit = r.left + window.scrollX < right && r.right + window.scrollX > left
                        && r.top + window.scrollY < bottom && r.bottom + window.scrollY > top
                    list[index].selected.val = additive ? (base[index] || hit) : hit
                })
            }

            const loop = () => {
                const edge = 60
                if (pointerY < edge) window.scrollBy(0, -Math.ceil((edge - pointerY) / 3))
                else if (pointerY > window.innerHeight - edge) window.scrollBy(0, Math.ceil((pointerY - (window.innerHeight - edge)) / 3))
                update()
                raf = requestAnimationFrame(loop)
            }

            const onMove = (ev: PointerEvent) => {
                pointerX = ev.clientX
                pointerY = ev.clientY
                if (!dragging) {
                    if (Math.hypot(ev.pageX - startX, ev.pageY - startY) < 6) return
                    dragging = true
                    box.style.display = 'block'
                    document.body.style.userSelect = 'none'
                    raf = requestAnimationFrame(loop)
                }
            }

            const onUp = () => {
                document.removeEventListener('pointermove', onMove)
                document.removeEventListener('pointerup', onUp)
                document.removeEventListener('pointercancel', onUp)
                cancelAnimationFrame(raf)
                box.style.display = 'none'
                document.body.style.userSelect = ''
                if (dragging) {
                    suppressClick = true
                    setTimeout(() => suppressClick = false, 0)
                }
            }

            document.addEventListener('pointermove', onMove)
            document.addEventListener('pointerup', onUp)
            document.addEventListener('pointercancel', onUp)
        })
    }

    return () => {
        const list = pages.val
        const container = div({ class: 'row gy-3 gx-3' },
            list.map((page, index) => {
                const badgeNotNum = !page.badge.match(/^\d+$/)
                const active = page.selected
                return div({ class: 'col-xxl-3 col-lg-4 col-md-6' },
                    div({
                        tabIndex: 0,
                        'data-item-index': index,
                        class: () => `${badgeNotNum
                            ? `vstack gap-2 justify-content-center`
                            : `hstack gap-3`
                            } shadow-sm h-100 text-break user-select-none card card-body video-item-btn bg-success bg-opacity-10 ${active.val ? 'active' : ''}`,
                        onclick(e: MouseEvent) {
                            onCardClick(list, index, e)
                        },
                        onkeyup(e: KeyboardEvent) {
                            if (e.key == 'Enter') {
                                onCardClick(list, index, e)
                            }
                        }
                    },
                        span({ class: 'badge text-bg-success bg-opacity-75 border', hidden: badgeNotNum }, page.badge),
                        div(page.part),
                        div({ class: `${page.part ? 'small text-muted' : ''}`, hidden: !badgeNotNum }, page.badge),
                    )
                )
            }),
        )
        enableBoxSelect(container, list)
        return container
    }
}

export default (
    workRoute: WorkRoute
) => new VideoItemListComp(workRoute).element