# 更新日志

本项目所有重要变更记录于此文件。

格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本号遵循语义化版本（SemVer）。

版本条目与 GitHub 标签一一对应：`v1.0.0` → `9341a8c`、`v1.0.1` → `fd0433b`、`v1.0.2` → `487f378`。

---

## [1.0.2] - 2026-08-24

### 修复
- **静默化试用期弹窗：提前 + 高频恢复 SLicense，堵住启动空窗**
  - **现象**：1.0.1 的 30 秒定时恢复能补回被清空的 SLicense，但仍有窗口期——Typora 启动瞬间、或二次验证刚清空后，弹窗逻辑先读到空值，弹出「试用期剩余 0 天」。
  - **根因**：恢复逻辑原先写在 `electron.app.whenReady()` 回调内部，要等 app 就绪才首次执行；且 30 秒的检查间隔远大于「二次验证清空 → 弹窗」之间的时间差。
  - **修复**（3 处）：
    1. `restoreSLicense()` 及其 `setInterval` 提到 `whenReady()` 之外，Hook 加载时同步立即执行一次，堵住「启动瞬间 → 首次恢复」的空窗。
    2. 检查间隔由 30 秒改为 2 秒。
    3. `browser-window-created` 的 `dom-ready` 回调中再补一次，确保渲染进程读到有效 license。

### 改动文件
| 文件 | 改动 |
|------|------|
| `typora_crack.js` | +24 / -17 行 |

### 提交信息
```
commit 487f3786a3ec79562139cc07b88fc49c7665d1ea
Author: hu <htryone@163.com>
Date:   Mon Aug 24 02:04:42 2026 +0800

静默化试用期弹窗：提前+高频恢复 SLicense，堵启动空窗
```

---

## [1.0.1] - 2026-07-26

### 修复
- **增加 SLicense 定时恢复机制，防止 Typora 的 2nd 二次验证清空 license**
  - **根因**：Typora 存在概率性 `2nd` 二次验证机制，在运行时于本地（纯 JS）验证 SLicense 的 RSA 签名。本项目写入的 SLicense 值（`RHJlYW1OeWE=#0#1/1/2059`，DreamNya 格式）并非有效 RSA 签名，验证失败后 Typora 执行 `onUnfillLicense` 清空注册表，导致每次启动都回到试用期倒计时。
  - **修复**：在注入的 Hook 中新增 `setInterval`，每 30 秒检查注册表 SLicense 值，一旦被清空立即自动写回；并在启动时立即检查一次作为兜底。

### 改动文件
| 文件 | 改动 |
|------|------|
| `typora_crack.js` | +18 行，新增 SLicense 定时恢复逻辑 |
| `.gitignore` | +1 行 |

### 提交信息
```
commit fd0433be2f78cf9893fa49f55be9589dce39e8b2
Author: hu <htryone@163.com>
Date:   Sun Jul 26 03:06:27 2026 +0800

修复: 增加 SLicense 定时恢复机制，防止 2nd 二次验证清空 license
```

---

## [1.0.0] - 2026-07-20

首个发行版对应的源码状态。

### 文档
- **README 适配发行版**：说明改为按压缩包内的文件路径书写，补充三步激活流程，保留 Typora 官网链接。

### 改动文件
| 文件 | 改动 |
|------|------|
| `README.md` | 文件路径、三步流程、官网链接 |

### 提交信息
```
commit 9341a8c9d21a99cf1529b217f75d8c911ba1b2fc
Author: hu <htryone@163.com>
Date:   Mon Jul 20 10:56:11 2026 +0800

docs: README 适配发行版（包内文件路径 + 三步流程 + 官网链接保留）
```
