# 项目指南

## 项目结构

- 调频 FM 是使用 Go 和 MyGo 原生 UI 的网络电台客户端，不使用 WebView。入口为 `main.go`。
- `internal/app`：界面、用户设置、收藏、会话、电台列表与更新入口。
- `internal/player`：播放、频谱和识曲；`internal/player/mpv` 负责动态加载随包 libmpv。
- `internal/radio`：Radio-Browser 目录查询与分页。
- `scripts`：固定音频组件的准备、便携包构建及其测试；`icon` 生成图标。

## 开发与验证

- 只新增覆盖当前改动关键行为、边界或缺陷的必要测试；优先复用现有测试，避免重复测试、无意义的覆盖率堆砌和仅验证实现细节的测试。
- Go 版本和 MyGo 工具版本以 `go.mod` 为准；Python 脚本使用 Python 3.11+。
- 首次运行开发版前执行 `python scripts/prepare_mpv.py`，然后运行 `go tool mygo dev`。原生窗口需要图形桌面环境。
- 常规验证：`go test ./...`、`go vet ./...`、`go build ./...`。
- 修改打包脚本后运行 `python -m unittest discover -s scripts -p 'test_*.py'`。
- 默认 Go 测试会跳过 libmpv 原生集成测试；涉及音频或打包时，在已准备本机组件后运行：

  ```bash
  TIAOPIN_TEST_MPV=1 TIAOPIN_MPV_LIBRARY="$PWD/resources/windows-amd64/mpv/mpv-2.dll" go test ./internal/... -count=1
  ```

  上例适用于 Windows Git Bash；其他平台使用对应架构的 `libmpv.dylib` 或 `libmpv.so.2`。未执行的跨平台、图形交互及原生测试须明确说明。

## 项目约束

- 播放和识曲使用固定的 libmpv 运行包；`scripts/mpv.lock.json` 是版本与 SHA-256 的真值。不要绕过校验、静默回退到系统库或在应用运行时下载组件。
- 不要把 `resources/<GOOS>-<GOARCH>/mpv/`、构建产物、用户数据或私钥提交到 Git。调整第三方依赖或分发内容时，检查 `THIRD_PARTY_NOTICES.md` 和许可材料；候选音频运行包的再分发审核尚需完成。
- Windows 便携版由 `portable.flag` 启用，用户数据位于程序目录的 `data/`；安装版使用系统用户数据目录。更新和打包不得覆盖或删除便携用户数据。
- 保持分页游标、首批缓存和筛选切换时的请求隔离；不要把网络错误伪装成空列表或成功结果。
- 保留原生更新包的签名验证和用户数据；不要提交、输出或更换更新私钥。开发版不具备正式更新能力。
- 修改 `icon/main.go` 后运行 `go run ./icon` 并检查生成文件；不要顺带修改无关平台图标。
- `README.md` 面向最终用户，保持为程序介绍；隐私、许可和更新日志分别维护在 `PRIVACY.md`、`THIRD_PARTY_NOTICES.md` 和 `CHANGELOG.md`。

## 发布

- `mygo.json` 的 `version`、`CHANGELOG.md` 的对应版本标题和 `v<version>` 标签必须一致。推送版本标签会触发 `.github/workflows/release.yml`，验证通过后自动公开 Release。
- 安装包当前未使用发布者签名，macOS 包未经公证；不要误称已签名或公证。更新包签名与系统代码签名是两回事。
- 提交、推送、打标签、发布或修改远端历史前遵循用户授权；不主动改写已发布标签。
