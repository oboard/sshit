import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'zh-CN',
  title: 'sshit',
  description: '一个端口同时提供 SSH 和共享 Web 终端工作区。',
  cleanUrls: true,
  head: [
    ['meta', { name: 'theme-color', content: '#0f172a' }],
    ['link', { rel: 'icon', href: '/favicon.svg' }],
  ],
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'sshit',
    nav: [
      { text: '指南', link: '/guide/getting-started' },
      { text: '配置', link: '/reference/configuration' },
      { text: 'GitHub', link: 'https://github.com/oboard/sshit' },
    ],
    sidebar: {
      '/guide/': [
        {
          text: '使用指南',
          items: [
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '协作工作区', link: '/guide/collaboration' },
            { text: '会话持久化', link: '/guide/persistence' },
            { text: '安全建议', link: '/guide/security' },
          ],
        },
      ],
      '/reference/': [
        {
          text: '参考',
          items: [
            { text: '命令行配置', link: '/reference/configuration' },
            { text: '架构与协议', link: '/reference/architecture' },
            { text: '从源码构建', link: '/reference/building' },
            { text: '窗口管理快捷键', link: '/reference/tiled-layout-shortcuts' },
          ],
        },
      ],
    },
    socialLinks: [{ icon: 'github', link: 'https://github.com/oboard/sshit' }],
    footer: {
      message: '以 GNU AGPLv3 许可证发布。',
      copyright: 'Copyright © sshit contributors',
    },
    outline: { label: '本页内容' },
    docFooter: { prev: '上一页', next: '下一页' },
    returnToTopLabel: '返回顶部',
    sidebarMenuLabel: '菜单',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
  },
})
