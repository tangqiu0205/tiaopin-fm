# 调频 FM · TiaopinFM

一款轻量的网络电台桌面客户端，使用原生界面，无 WebView。发现世界各地的电台，收藏喜欢的声音，随时打开收听。

## 功能

- **发现电台**：按内容、地区、语言和名称筛选，滚动加载更多电台。
- **随机播放**：掷一颗骰子，从当前列表随机选台，立即开播。
- **收藏与记录**：保存收藏、最近收听和上次播放的电台。
- **听歌识曲**：识别正在播放的歌曲，保存识曲历史，支持复制歌曲信息。
- **定时停止**：选择预设时长或自定义时间，到时自动停止播放。
- **主题切换**：支持浅色、深色和跟随系统。
- **后台收听**：通过系统托盘控制播放，无需一直打开主窗口。
- **应用内更新**：检查新版本、查看更新说明，下载并验证签名后更新。

内置音频播放组件，无需另行安装 mpv 或 FFmpeg，支持 MP3、AAC / HE-AAC、Vorbis、Opus、FLAC 和 HLS 等格式。

## 下载

前往 [GitHub Releases](https://github.com/tangqiu0205/tiaopin-fm/releases) 下载：

| 平台 | 架构 | 格式 |
| --- | --- | --- |
| Windows | x64、ARM64 | 安装包、便携 ZIP |
| macOS 15+ | Intel、Apple Silicon | DMG |
| Linux | x64、ARM64 | DEB、tar.gz |

Windows 便携版将用户数据保存在程序目录的 `data` 文件夹中，方便随程序一起移动和备份。系统包安装或目录不可写时，需手动更新。

安装包尚未使用发布者签名，macOS 包未经公证，系统可能提示风险或阻止打开，请确认下载来源后再决定是否运行。

## 说明

电台目录由 [Radio-Browser](https://www.radio-browser.info) 提供，电台内容及版权归各权利人所有。听歌识曲使用非官方 Shazam 接口；第三方电台与识曲服务的可用性不作保证。

本项目基于 [EGOIST](https://github.com/egoist) 开发的 Go / MyGo 音乐客户端改造，使用 [MyGo](https://github.com/egoist/mygo) 构建，遵循 [AGPL-3.0](LICENSE) 开源协议，并保留上游版权。

[隐私说明](PRIVACY.md) · [第三方声明](THIRD_PARTY_NOTICES.md) · [更新日志](CHANGELOG.md)
