# 开发文档（DOC）

这套文档面向**想自己改、自己构建发行包**的人。
只想用的话看仓库根 [README](../README.md) 和绿色包里的 `README-先看我.txt` / `使用说明.md`。

## 按需求读

| 我想…… | 看这篇 |
|---|---|
| 从零造一个「免安装绿色版」 | [BUILD-GREEN.md](BUILD-GREEN.md) |
| 配一个「脚本版」发行包 | [BUILD-SCRIPT.md](BUILD-SCRIPT.md) |
| 改绿色版的启动器（那个 exe） | [LAUNCHER.md](LAUNCHER.md) |
| 发版、上传、更新说明 | [RELEASE.md](RELEASE.md) |
| 弹试用 / 激活失败，要排错 | [TROUBLESHOOTING.md](TROUBLESHOOTING.md) |
| 搞懂注入到底干了什么 | 仓库根 [TECHNICAL.md](../TECHNICAL.md)（较早，部分已过时）+ `typora_crack.js` 里的注释 + [CHANGELOG.md](../CHANGELOG.md)（每处修复都写了根因与验证） |

## 仓库结构

```
typora_crack.js          补丁主脚本；注入模板（getInsertCode）就在里面
start.bat                脚本版入口：自动找 Node，跑补丁脚本
scripts/
  build-launcher.sh      编译绿色版启动器
  launcher/Program.cs    启动器源码（C#，只用 .NET Framework 自带库）
DOC/                     本文档目录
CHANGELOG.md             版本变更 + 根因 + 验证记录（最有信息量的一篇）
TECHNICAL.md             注入原理（历史文档）
```

不进仓库（`.gitignore`）：`node_modules/`、`fax/`（本地发行母本）、`green-test/`（绿色版成品目录）、`*.7z`。

## 贡献约定

- 提交信息用**简体中文**；一次提交只做一件事，别把无关改动混进来。
- 改了 `typora_crack.js` 的注入模板 → 必须 `node --check` 通过 + 实机跑一次，并在 CHANGELOG 里写「怎么验证的」。
- 别提交：任何凭据/令牌、`node_modules/`、发行包 7z、打包机上的绝对路径。
- 提 issue 请带上：Typora 版本号（关于 Typora 里看）、用的是绿色版还是脚本版、`%APPDATA%\Typora\typora.log` 里的相关片段。
- 界面/脚本里给最终用户看的文字：`.bat` 只用 ASCII；`.vbs` 存 UTF-16；`.md`/`.txt` 用 UTF-8（包内 `README-先看我.txt` 沿用带 BOM，方便老记事本）。

## 免责声明

本项目仅供学习与技术研究，请于 24 小时内自行删除；请支持正版 [Typora](https://typora.io)。
