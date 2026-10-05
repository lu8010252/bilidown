/// <reference types="vite/client" />
import van from 'vanjs-core'
import Header from './header'
import Work from './work'
import Task from './task'
import Login from './login'
import Setting from './setting'
import _Error from './error'
import { redirect } from 'vanjs-router'
import { GLOBAL_HIDE_PAGE } from './mixin'
import 'bootstrap/dist/css/bootstrap.min.css'
import './scss/index.scss'
import './scss/appearance.scss'
import { initAppearance } from './appearance'

const { div } = van.tags

// 先应用主题和背景，避免页面先闪一下默认配色
initAppearance()

redirect('home', 'work')

van.add(document.body,
    div({ class: 'container py-4 vstack gap-4', hidden: GLOBAL_HIDE_PAGE },
        Header(),
        Work(),
        Task(),
        Login(),
        Setting(),
    ),
    _Error()
)
