# CI / PR 流程

从最新 `main` 建短期分支，以 Draft PR 记录工作与验证范围；准备审阅时转为 ready。此模块很小，Draft 与 ready 均运行一次 `go test -race -vet=off -count=1 ./...` 和一次 `go vet ./...`。无需为提交文档或创建 PR 在本地反复重跑测试。

CI 只监听 PR、`main` push 和手动触发，不监听普通功能分支 push。同一 PR 的新提交会取消旧运行；`ready_for_review` 会启动一次最终检查。Go 版本读取 `go.mod`（当前为 1.27.x），使用原生 CGO/C 编译器；Git 完整 checkout，不保留凭据。依赖与构建缓存以 `go.mod` 为键，依赖预热后检查清单未改变；当前无外部模块依赖、无 `go.sum`。

`CI summary` 总会运行并汇总 race / vet 结果；后续若维护者配置 required checks，应选择这个稳定检查名。先看最终提交的状态摘要，失败时才阅读具体 job 日志。最终 SHA 通过后停止重复验证。PR 运行检查的是合成 merge commit，摘要同时标明 PR head SHA。

首次加入工作流先由本 PR 的 `pull_request` 事件验证。手动触发要求工作流已存在于默认分支；若仓库设置阻止首次运行，记录实际阻碍并由维护者处理。此变更不修改分支保护、Actions 计费或安全设置，不合并或发布。
