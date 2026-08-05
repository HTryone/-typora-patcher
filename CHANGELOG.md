# 更新日志

本项目所有重要变更记录于此文件。

格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本号遵循语义化版本（SemVer）。

---

## [1.0.0] - 2026-07-26

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
