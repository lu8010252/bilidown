import van from 'vanjs-core'

/** 主题模式：浅色 / 深色 / 日出日落自动 / 跟随系统 */
export type ThemeMode = 'light' | 'dark' | 'sun' | 'system'

export type Appearance = {
    theme: ThemeMode
    /** 背景配色 id，见 `PALETTES` */
    palette: string
    /** 是否开启玻璃效果 */
    glass: boolean
    /** 玻璃模糊强度，px */
    blur: number
    /** 玻璃面板不透明度，百分比 */
    opacity: number
    /** 日出日落计算所用的纬度、经度 */
    lat: number
    lng: number
}

export type Palette = {
    id: string
    name: string
    /** 渐变色停靠点；为 null 表示纯色（使用 Bootstrap 默认背景） */
    light: string[] | null
    dark: string[] | null
}

export const PALETTES: Palette[] = [
    { id: 'aqua-rose', name: '青粉', light: ['#d4f1f4', '#e8e4f8', '#fde2ec'], dark: ['#0f2a33', '#1f1c35', '#331a29'] },
    { id: 'ocean', name: '海盐蓝', light: ['#dff1ff', '#e6ecff', '#d9f7ff'], dark: ['#0b1d33', '#131c3a', '#0b2d36'] },
    { id: 'dusk', name: '暮光紫', light: ['#ece4ff', '#f6e3ff', '#ffe3ef'], dark: ['#1a1233', '#281340', '#3a1530'] },
    { id: 'mint', name: '薄荷绿', light: ['#dcf8e8', '#e3f6f1', '#f1f9d9'], dark: ['#0c2a1d', '#0f2b2b', '#232a0f'] },
    { id: 'sunset', name: '暖橙', light: ['#fff0dc', '#ffe4d9', '#ffe1ec'], dark: ['#33200c', '#341a14', '#331424'] },
    { id: 'plain', name: '纯色', light: null, dark: null },
]

export const THEME_ORDER: ThemeMode[] = ['light', 'dark', 'sun', 'system']

export const THEME_LABELS: Record<ThemeMode, string> = {
    light: '☀️ 浅色',
    dark: '🌙 深色',
    sun: '🌅 日出日落',
    system: '💻 跟随系统',
}

/** 渐变背景的 CSS 值；纯色返回 `none` */
export const gradientOf = (palette: Palette, dark: boolean) => {
    const colors = dark ? palette.dark : palette.light
    return colors ? `linear-gradient(135deg, ${colors.join(', ')})` : 'none'
}

const STORAGE_KEY = 'bilidown.appearance'

/** 默认坐标：安徽蚌埠怀远。可在设置里修改，只用来估算日出日落时刻。 */
const DEFAULTS: Appearance = {
    theme: 'light',
    palette: 'aqua-rose',
    glass: true,
    blur: 14,
    opacity: 55,
    lat: 32.97,
    lng: 117.18,
}

const clamp = (value: unknown, min: number, max: number, fallback: number) => {
    const n = Number(value)
    return Number.isFinite(n) ? Math.min(max, Math.max(min, n)) : fallback
}

/** 读取本浏览器保存的外观设置；存储不可用或内容损坏时回退默认值 */
const load = (): Appearance => {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        if (!raw) return { ...DEFAULTS }
        const data = JSON.parse(raw) as Partial<Appearance>
        return {
            theme: THEME_ORDER.includes(data.theme as ThemeMode) ? data.theme as ThemeMode : DEFAULTS.theme,
            palette: PALETTES.some(p => p.id === data.palette) ? data.palette as string : DEFAULTS.palette,
            glass: typeof data.glass === 'boolean' ? data.glass : DEFAULTS.glass,
            blur: clamp(data.blur, 0, 30, DEFAULTS.blur),
            opacity: clamp(data.opacity, 30, 95, DEFAULTS.opacity),
            lat: clamp(data.lat, -90, 90, DEFAULTS.lat),
            lng: clamp(data.lng, -180, 180, DEFAULTS.lng),
        }
    } catch {
        return { ...DEFAULTS }
    }
}

const save = (value: Appearance) => {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
    } catch { /* 隐私模式等场景下不可用，本次会话内仍然生效 */ }
}

export const appearance = van.state<Appearance>(load())

/** 当前实际生效的是否为深色（由主题模式解析而来） */
export const resolvedDark = van.state(false)

const SPAN = 86400000

/**
 * 估算某天的日出日落时刻（NOAA 简化算法，误差在几分钟内）。
 * 极昼、极夜地区返回 null。
 */
export const sunTimes = (date: Date, lat: number, lng: number): { rise: Date, set: Date } | null => {
    const rad = Math.PI / 180
    const start = Date.UTC(date.getFullYear(), 0, 0)
    const today = Date.UTC(date.getFullYear(), date.getMonth(), date.getDate())
    const dayOfYear = Math.round((today - start) / SPAN)
    const lngHour = lng / 15
    const norm = (x: number, m: number) => ((x % m) + m) % m
    // 以本地正午为基准，把 UT 小时换算成离正午最近的真实时刻，避免跨日错位
    const noon = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 12).getTime()
    const toInstant = (ut: number) => {
        const base = today + norm(ut, 24) * 3600000
        return [base - SPAN, base, base + SPAN].reduce((a, b) => Math.abs(b - noon) < Math.abs(a - noon) ? b : a)
    }

    const calc = (rising: boolean): Date | null => {
        const t = dayOfYear + ((rising ? 6 : 18) - lngHour) / 24
        const M = 0.9856 * t - 3.289
        const L = norm(M + 1.916 * Math.sin(M * rad) + 0.02 * Math.sin(2 * M * rad) + 282.634, 360)
        let RA = norm(Math.atan(0.91764 * Math.tan(L * rad)) / rad, 360)
        RA += Math.floor(L / 90) * 90 - Math.floor(RA / 90) * 90
        RA /= 15
        const sinDec = 0.39782 * Math.sin(L * rad)
        const cosDec = Math.cos(Math.asin(sinDec))
        const cosH = (Math.cos(90.833 * rad) - sinDec * Math.sin(lat * rad)) / (cosDec * Math.cos(lat * rad))
        if (cosH > 1 || cosH < -1) return null
        const H = (rising ? 360 - Math.acos(cosH) / rad : Math.acos(cosH) / rad) / 15
        const T = H + RA - 0.06571 * t - 6.622
        return new Date(toInstant(T - lngHour))
    }

    const rise = calc(true)
    const set = calc(false)
    return rise && set ? { rise, set } : null
}

/** 现在是否处于夜间；无法计算日出日落时按 6:00–18:00 兜底 */
const isNight = (lat: number, lng: number, now = new Date()) => {
    const sun = sunTimes(now, lat, lng)
    if (sun) return now < sun.rise || now >= sun.set
    const hour = now.getHours()
    return hour < 6 || hour >= 18
}

const systemQuery = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null

/** 把当前设置应用到页面：主题、背景渐变、玻璃参数 */
export const applyAppearance = () => {
    const a = appearance.val
    const dark = a.theme === 'dark' ? true
        : a.theme === 'light' ? false
            : a.theme === 'system' ? !!systemQuery?.matches
                : isNight(a.lat, a.lng)
    resolvedDark.val = dark

    const root = document.documentElement
    root.setAttribute('data-bs-theme', dark ? 'dark' : 'light')
    const palette = PALETTES.find(p => p.id === a.palette) ?? PALETTES[0]
    root.style.setProperty('--app-bg', gradientOf(palette, dark))
    root.style.setProperty('--glass-blur', `${a.blur}px`)
    root.style.setProperty('--glass-alpha', String(a.opacity / 100))
    root.classList.toggle('glass-on', a.glass)
}

export const setAppearance = (patch: Partial<Appearance>) => {
    appearance.val = { ...appearance.val, ...patch }
    save(appearance.val)
    applyAppearance()
}

/** 在渲染页面之前调用，避免先闪一下默认配色；并开启日出日落/跟随系统的自动切换 */
export const initAppearance = () => {
    applyAppearance()
    // 日出日落：每分钟检查一次是否跨过日出或日落
    setInterval(() => {
        if (appearance.val.theme === 'sun') applyAppearance()
    }, 60000)
    systemQuery?.addEventListener('change', () => {
        if (appearance.val.theme === 'system') applyAppearance()
    })
}
