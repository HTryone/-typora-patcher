# 开发文档（DOC）

这套文档面向**想自己改、自己构建发行包**的人。
只想用的话看仓库根 [README](../README.md) 和绿色包里的 `README-先看我.txt` / `使用说明.md`。

## 按需求读

| 我想…… | 看这篇 |
|---|---|
| 从零造一个「免安装绿色版」 | [BUILD-GREEN.md](BUILD-GREEN.md) |
| 自用组装「脚本版」包（**已停止独立发行**，不上 Release） | [BUILD-SCRIPT.md](BUILD-SCRIPT.md) |
| 压缩参数、资产命名、打包前清点 | [PACKAGING.md](PACKAGING.md) |
| 改绿色版的启动器（那个 exe） | [LAUNCHER.md](LAUNCHER.md) |
| 发版、上传、更新说明 | [RELEASE.md](RELEASE.md) |
| 弹试用 / 激活失败，要排错 | [TROUBLESHOOTING.md](TROUBLESHOOTING.md) |
| 想知道「某一版 Typora 凭什么认为能改」、升级后要重查什么 | [COMPATIBILITY.md](COMPATIBILITY.md)（逐版本核查证据 + 尚未验证的项） |
| 想看 1.14.10 授权链路的**完整逆向**（验签/续期/指纹/密钥存放） | [REVERSE-1.14.10/](REVERSE-1.14.10/REVERSE-1.14.10.md)（含分析脚本与全量字符串表存档） |
| 搞懂注入到底干了什么 | 仓库根 [TECHNICAL.md](../TECHNICAL.md)（较早，部分已过时）+ `script-release\typora_crack.js` 里的注释 + [CHANGELOG.md](../CHANGELOG.md)（每处修复都写了根因与验证） |

## 仓库结构

```
script-release/          脚本版（start.bat + typora_crack.js + 依赖清单；**不再发行**，注入模板 getInsertCode 在 typora_crack.js 里）
typoraGO/                Go 版激活工具（图形界面；详见其 README）
scripts/
  build-launcher.sh      编译绿色版启动器
  launcher/Program.cs    启动器源码（C#，只用 .NET Framework 自带库）
DOC/
  REVERSE-1.14.10/       1.14.10 授权链路逆向报告 + 分析脚本 + 字符串表存档
CHANGELOG.md             版本变更 + 根因 + 验证记录（最有信息量的一篇）
TECHNICAL.md             注入原理（历史文档）
```

不进仓库（`.gitignore`）：`node_modules/`、`fax/`（本地发行母本）、`green-test/`（绿色版成品目录）、`dist/`（发行二进制）、`tmp*`。

> **脚本版定位**：`typora_crack.js` / `start.bat` / `package.json` 已迁到仓库内 `script-release/` 归类，**不再打包发行**；后期维护重心是绿色版与 typoratool。

> **依赖清单要跟着改**：`package.json` / `package-lock.json` 是**源码的一部分**，换依赖、升依赖都要一起提交，否则别人 clone 下来跑 `npm install` 会装到另一套版本（`@electron/fuses` 尤其敏感，它要求 Node ≥ 22.12）。脚本首次运行时的自动安装见 [TECHNICAL.md](../TECHNICAL.md)。

## 贡献约定

- 提交信息用**简体中文**；一次提交只做一件事，别把无关改动混进来。
- 改了 `script-release\typora_crack.js` 的注入模板 → 必须 `node --check` 通过 + 实机跑一次；Go 版对齐测试（`TestHookMatchesJSScript`）也要重跑，两边注入代码必须逐字节一致。
- 别提交：任何凭据/令牌、`node_modules/`、发行包 7z、打包机上的绝对路径。
- 提 issue 请带上：Typora 版本号（关于 Typora 里看）、用的是绿色版还是脚本版、`%APPDATA%\Typora\typora.log` 里的相关片段。
- 界面/脚本里给最终用户看的文字：`.bat` 只用 ASCII；`.vbs` 存 UTF-16；`.md`/`.txt` 用 UTF-8（包内 `README-先看我.txt` 沿用带 BOM，方便老记事本）。

## 免责声明

本项目仅供学习与技术研究，请于 24 小时内自行删除；请支持正版 [Typora](https://typora.io)。
