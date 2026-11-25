# 脚本版（JS）—— 归类维护目录

> **定位（维护者决定，2026-11）**：脚本版**退出发行体系**，整个目录随仓库归类维护——
> 代码在这里继续更新，但**不打包、不上 Release、不写 CHANGELOG 版本条目**。
> 后期维护重心：**离线绿色版 + typoratool（Go 版）**；脚本版太麻烦（手装 Node/手选路径/手贴机器码），不再单独发布。
> 需要给别人时：从这个目录组装压包私下发即可。
>
> 公开 Release 只有：`Typora.Green.7z` + `typoratool-vX.Y.Z.exe`（见 [../DOC/RELEASE.md](../DOC/RELEASE.md)）。

## 目录内容（已从仓库根迁入此处）

| 文件/目录 | 说明 |
|---|---|
| `start.bat` | 用户双击的入口（自动找/装 Node，再跑补丁） |
| `typora_crack.js` | 补丁主脚本（注入模板仍是 Go 版对齐的基准，改它要跑 Go 版对齐测试） |
| `package.json` / `package-lock.json` | 依赖锁定（asar 3.2.0 / chalk 4.1.2 / readline-sync 1.4.10 / iconv-lite 0.7.2 / @electron/fuses 2.1.2，Node ≥ 22.12） |
| `node_modules/` | 首次跑 `start.bat` 自动生成（gitignore） |
| `fax/` | Node.js 安装包母本（组装发行包时用，gitignore，硬链接要求同分区） |

打包细节（5 项清单、NTFS 硬链接、`发行说明.txt` 的 UTF-8 BOM + CRLF 约定、打包前自查）见 [../DOC/BUILD-SCRIPT.md](../DOC/BUILD-SCRIPT.md)。
