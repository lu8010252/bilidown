import van from 'vanjs-core'
import {
    PALETTES, THEME_LABELS, THEME_ORDER, appearance, gradientOf, resolvedDark, setAppearance, sunTimes,
} from '.'

const { button, div, input, label, span } = van.tags

/** 页头右侧的快速主题切换按钮：点击在 浅色 → 深色 → 日出日落 → 跟随系统 之间循环 */
export const ThemeToggle = () => button({
    class: 'btn btn-sm btn-outline-secondary text-nowrap flex-shrink-0 ms-auto',
    title: '点击切换主题',
    onclick() {
        const index = THEME_ORDER.indexOf(appearance.val.theme)
        setAppearance({ theme: THEME_ORDER[(index + 1) % THEME_ORDER.length] })
    }
}, () => THEME_LABELS[appearance.val.theme])

const pad = (n: number) => String(n).padStart(2, '0')
const hhmm = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`

/** 设置中心里的“外观设置”面板。设置只保存在当前浏览器中。 */
export const AppearanceSetting = () => {
    const sunInfo = () => {
        const { lat, lng } = appearance.val
        const sun = sunTimes(new Date(), lat, lng)
        return sun
            ? `今日日出 ${hhmm(sun.rise)}，日落 ${hhmm(sun.set)}，日落后自动切换为深色`
            : '该坐标今天没有日出日落（极昼/极夜），按 6:00–18:00 切换'
    }

    const coordInput = (key: 'lat' | 'lng', name: string, min: number, max: number) => div({ class: 'input-group input-group-sm', style: 'max-width: 190px' },
        span({ class: 'input-group-text' }, name),
        input({
            class: 'form-control', type: 'number', step: '0.01', min, max,
            value: () => appearance.val[key],
            onchange: (event: Event) => {
                const n = Number((event.target as HTMLInputElement).value)
                if (Number.isFinite(n) && n >= min && n <= max) setAppearance({ [key]: n })
                else (event.target as HTMLInputElement).value = String(appearance.val[key])
            }
        })
    )

    const slider = (name: string, key: 'blur' | 'opacity', min: number, max: number, unit: string) => div({ class: 'hstack gap-3' },
        span({ class: 'text-nowrap', style: 'width: 5em' }, name),
        input({
            class: 'form-range', type: 'range', min, max, step: 1,
            value: () => appearance.val[key],
            oninput: (event: Event) => setAppearance({ [key]: Number((event.target as HTMLInputElement).value) })
        }),
        span({ class: 'text-secondary text-nowrap', style: 'width: 4em' }, () => `${appearance.val[key]}${unit}`)
    )

    return div({ class: 'app-panel p-3 vstack gap-3' },
        div({ class: 'fw-bold' }, '外观设置'),

        div({ class: 'vstack gap-2' },
            span({ class: 'text-secondary small' }, '主题'),
            div({ class: 'btn-group flex-wrap', role: 'group' },
                THEME_ORDER.map(mode => button({
                    type: 'button',
                    class: () => `btn btn-sm ${appearance.val.theme === mode ? 'btn-primary' : 'btn-outline-primary'}`,
                    onclick: () => setAppearance({ theme: mode }),
                }, THEME_LABELS[mode]))
            ),
            () => appearance.val.theme === 'sun'
                ? div({ class: 'vstack gap-2' },
                    div({ class: 'text-secondary small' }, sunInfo()),
                    div({ class: 'hstack gap-2 flex-wrap' },
                        coordInput('lat', '纬度', -90, 90),
                        coordInput('lng', '经度', -180, 180),
                        span({ class: 'text-secondary small' }, '默认为安徽怀远，按你所在位置修改可让时刻更准'),
                    ))
                : ''
        ),

        div({ class: 'vstack gap-2' },
            span({ class: 'text-secondary small' }, '背景配色'),
            div({ class: 'hstack gap-3 flex-wrap' },
                PALETTES.map(palette => button({
                    type: 'button',
                    class: () => `swatch ${appearance.val.palette === palette.id ? 'active' : ''}`,
                    title: palette.name,
                    onclick: () => setAppearance({ palette: palette.id }),
                },
                    div({
                        class: 'swatch-dot',
                        style: () => `background: ${gradientOf(palette, resolvedDark.val)}`,
                    }),
                    div({ class: 'small' }, palette.name),
                ))
            )
        ),

        div({ class: 'vstack gap-2' },
            div({ class: 'form-check form-switch' },
                input({
                    class: 'form-check-input', type: 'checkbox', role: 'switch', id: 'glass-switch',
                    checked: () => appearance.val.glass,
                    onchange: (event: Event) => setAppearance({ glass: (event.target as HTMLInputElement).checked }),
                }),
                label({ class: 'form-check-label', for: 'glass-switch' }, '玻璃效果（半透明 + 背景模糊）'),
            ),
            () => appearance.val.glass
                ? div({ class: 'vstack gap-1' },
                    slider('模糊强度', 'blur', 0, 30, 'px'),
                    slider('面板浓度', 'opacity', 30, 95, '%'))
                : ''
        ),

        div({ class: 'text-secondary small' }, '外观设置保存在当前浏览器中，换设备或清除浏览器数据后需要重新设置。'),
    )
}
