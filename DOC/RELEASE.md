# 发版与上传

本页是**公开版**流程（可以照着做）。维护者本地还有一份更细的私有 SOP（含本地路径等），内容与本页一致、以本页为准。

## 0. 准备

```bash
git status          # 干净，或只有本次预期的改动
git log --oneline -3
```

改了什么，就同步更新什么：

| 改了 | 要同步 |
|---|---|
| 注入模板 / 启动器 / 打包规则 | [CHANGELOG.md](../CHANGELOG.md) 的版本条目（写清**根因 + 怎么验证的**） |
| 用户能感知的行为（入口、参数、界面） | 仓库根 [README.md](../README.md)、包内 `README-先看我.txt`、`使用说明.md` |
| 一条新经验/坑 | `DOC/` 里对应那篇 |

## 1. 提交

```bash
git add <显式写路径>        # 建议显式列出，别用 git add -A，免得把临时脚本带进去
git commit -m "fix: 一句话说清这次修了什么"
```

- 提交信息**用简体中文**；一次提交只做一件事；
- `git status` 里出现 `?? tmp_*` 之类的临时文件 → 先删掉再提交；一次性的临时脚本统一放 `.workbuddy\.temp\`（已在 `.gitignore` 里），文件会被忽略。

## 2. 版本与标签

```bash
git tag -a vX.Y.Z -m "typora-patcher-X.Y.Z"
```

- 版本号遵循 SemVer（修 bug 进 patch 位）；
- CHANGELOG 的版本条目要和标签**一一对应**；如果是「修复 + 更新日志」合并成单提交发版，条目里写「哈希以 `git rev-parse vX.Y.Z` 为准」，别写死会变的短哈希。

## 3. 推

```bash
git push                    # 普通推送
git push --set-upstream origin main   # 首次或上游跟踪丢了时
```

**改写历史（`--force-with-lease`）属于破坏性操作：必须由维护者单独确认后再执行**，别顺手推。

## 4. 打包

- 脚本版 → [BUILD-SCRIPT.md](BUILD-SCRIPT.md)
- 绿色版 → [BUILD-GREEN.md](BUILD-GREEN.md)（记得先 `bash scripts/build-launcher.sh` 重编启动器）

## 5. 上传 Release

资产名固定这两个，别人也好写脚本：

| 资产 | 内容 |
|---|---|
| `typora-patcher-X.Y.Z.7z` | 脚本版包 |
| `Typora.Green.7z` | 绿色版包 |

做法（最简单、最不容易错）：GitHub → Releases → 选中该 tag → **Edit** → 把新包拖进 assets → **Update release**。

- 想**替换**旧资产（同一版本重发）：先删掉同名资产，再传新的；
- 想**保留**旧包：先给旧资产改名（如 `...-old.7z`）再传；
- 命令行上传要读写二进制 body，本仓库的工具脚本目前只覆盖 JSON API；如确实需要命令行上传，先给工具脚本加一个 asset-upload 子命令（改公共脚本前先征得同意）。

## 6. 发完自查

- [ ] 用浏览器**下载自己刚传的包**，解压 → 双击 → 能正常激活/能建快捷方式；
- [ ] Release 页面上的资产名、说明、版本号对得上；
- [ ] README 顶部的「本版更新」不是上一版的陈词；
- [ ] 包内说明文件的版本号与包名一致。

## 凭据纪律

- 令牌、账号、密码**一律不进仓库、不进包、不写进任何说明文件**；
- 命令行取令牌只在本地凭据管理器里取（本项目使用的工具脚本就是这么做的，且不回显）。
