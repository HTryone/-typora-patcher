# 更新日志

本项目所有重要变更记录于此文件。

格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本号遵循语义化版本（SemVer）。

版本条目与 GitHub 标签一一对应：`v1.0.0` → `9341a8c`、`v1.0.1` → `fd0433b`、`v1.0.2` → `487f378`；`v1.0.3` 起修复与更新日志合并为单提交，以标签指向为准（`git rev-parse v1.0.3`）。

---

## [1.0.6] - 2026-10-24

### 修复
- **绿色版换机器必回「未激活 + 15 天试用」（issue #2 用户实测）—— 根因是 fingerprint 覆盖的时序竞态**
  - **现象**：别人机器上解压绿色版、双击 `START-Typora.bat`，始终未激活；日志里 `SLicense` 有值、`pure = undefined` 之后直接 `no info` → `onUnfillLicense`，而 `[machineCode] <本机指纹>` 这一行出现在 `no info` **之后**。
  - **根因**：`license.machineCode` 是**异步**处理器（返回 Promise），Typora 的首次许可校验却是**同步**取 fingerprint 的。v1.0.5 只在 5 个 hook 里临时调一次 `__getRealFp()`，永远慢半拍 —— 实体构建时 `__realFp` 仍是 null，只能用「打包时烘焙进文件的指纹」，于是**只有打包机（烘焙值＝本机值）能通过**，换机器必然 `no info`。
  - **对照实验**（本机、全新 profile）：只把烘焙指纹改成假值 `AAAAAAAAAA`，立刻复现用户那种 `pure = undefined → no info → onUnfillLicense`，且 `[machineCode]` 在 35 行之后才出现 —— 与系统/环境无关，纯时序问题。
  - **修复**（`getInsertCode` 注入模板）：
    1. `const electron = require("electron")` 之后新增 `__warmRealFp()`：在 jsc 加载前就每 8ms 轮询，`license.machineCode` 一注册就立刻取本机指纹并缓存（实测 70–80ms 就位，首次校验在 250–310ms，稳赢）；
    2. 取到后把指纹写入 `HKCU\Software\Typora\MFp`，之后每次启动**同步命中**缓存，彻底消除竞态（指纹跨机器固定、跨版本稳定）；
    3. 原 `__getRealFp()` 懒取路径保留作兜底。
  - **验证**（本机三种场景，以 `typora.log` 为准）：
    - 烘焙指纹改假 + 全新 profile + 无缓存 → `handle license.machineCode`(75ms) → `[machineCode]`(79ms) → `renew` → **`[L] pass`**；
    - 烘焙指纹与 deviceId **同时**改成别人机器的值（`AAAAAAAAAA` / `OTHERS-PC`）+ 全新 profile + 无缓存 → **`[L] pass`**（再次确认 deviceId 不参与比对）；
    - 真实路径（`START-Typora.bat` + 本机 profile，走注册表缓存）→ **`[L] pass`**。

### 修复（绿色版启动器）
- **`START-Typora.bat` 首次运行不建快捷方式**：原来用「包内是否存在 `Typora Green.lnk`」判断首次运行，而这个 .lnk 正是打包前本地跑一次留下的、会随包发行 → 用户机永远跳过建快捷方式的分支（与 issue #2 用户「桌面无快捷方式、`SETUP.bat` 反而能建」完全吻合）。改为用包**外**标记 `%LOCALAPPDATA%\TyporaGreen.setup-done` 判断，并在 PowerShell 失败时提示改用 `SETUP.bat`。

### 改动（绿色版启动器换成编译版 exe）
- **新增 `START-Typora.exe`**：用系统自带 .NET Framework 4.x 的 `csc.exe` 编译（源码 `scripts/launcher/Program.cs`，构建脚本 `scripts/build-launcher.sh`），**产物 6.5KB、零额外依赖**，Win10/11 直接可跑。
  - 旧的 `START-Typora.bat` 是「.bat 里塞一长串内联 PowerShell」：引号、`%` 展开、`&|()` 转义、编码四件事一起踩，而且失败还是静默的（本次「用户机不建快捷方式」就是这么跳过去的）。
  - exe 改用 `WScript.Shell` COM 直接建 `.lnk`，**不受 PowerShell 执行策略、也不受「压缩包解压带网络标记（MOTW）」影响**；桌面路径按注册表 `User Shell Folders` 解析（兼容桌面被重定向到 `D:\desktop` 这类情况）；出错用 MessageBox 明确弹出，不再静默。
  - **`.bat` 入口不再随包提供**：包内唯一入口就是 `START-Typora.exe`。旧的 `START-Typora.bat` 只做「标记不存在才建快捷方式」、没有自检自愈，内联 PowerShell 写法也不稳；exe 若被安全软件误拦，处置办法是加入白名单。
  - **快捷方式自愈**：标记做成三态 —— 不存在 = 首次运行；`done` = 已装；`removed` = 用户在 `SETUP.bat` 选过 3（Remove everything）。`done` 状态下每次启动都自检两处快捷方式：**文件被删了、或 target 指向的不是当前这个包（文件夹被挪走），就自动补建 / 改成新路径**，不用手动折腾。`SETUP.bat` 的 3 号选项随之补一行写入 `removed`，否则启动器会把刚删掉的图标又建回来。
  - **实测矩阵**（本机，日志/快捷方式 target 为准）：① 无标记无快捷方式 → 两处都建、标记写 `done`；② 标记 `done` 但桌面 .lnk 被删 → 自动补建；③ 整个包目录改名 → 两处快捷方式自动改指新路径；④ 标记 `removed` → 一处都不建（尊重「Remove everything」）；⑤ 真实包跑一遍 → `[L] pass`，两处快捷方式指向正确。

### 改动文件
| 文件 | 改动 |
|------|------|
| `typora_crack.js` | 注入模板 +37 行：`__warmRealFp` 预热轮询 + 注册表缓存 `MFp` |
| `scripts/launcher/Program.cs`（新） | 绿色版启动器源码（C#，.NET Framework 4.x，仅用系统库） |
| `scripts/build-launcher.sh`（新） | 构建脚本：`bash scripts/build-launcher.sh` → `green-test/START-Typora.exe` |
| `green-test/START-Typora.exe`（新，不进 git） | 编译版启动器，6.5KB |
| `green-test/START-Typora.bat`（**已删除**，不进 git 成品） | 该入口已移除，包内只保留 `START-Typora.exe` |
| `green-test/app/resources/app/launch.dist.js`（不进 git） | 绿色版成品注入文件重新生成（含上述修复） |
| `green-test/README-先看我.txt`、`green-test/使用说明.md` | 入口改为 `START-Typora.exe`，说明后备入口与误报处置 |
| `green-test/Typora Green.lnk`（删除，不进 git） | 本机跑启动器时生成的快捷方式，指向打包机绝对路径，不该随包发行 |
| `green-test/app/typora_crack.js`（删除，不进 git） | 构建残留的旧 patcher 副本，运行时不引用，留着会误导使用者 |
| `green-test/app/使用说明.md`（删除，不进 git） | 早期遗留的陈旧副本，内容已过时（提到不存在的 `bat\register-assoc.bat`、说双击 START-Typora.bat），包根已有正确版本 |
| `DOC/`（新） | 6 篇开发者文档：索引、构建绿色版、配脚本版包、启动器开发、发版上传、排错手册 |

### 提交

| 提交 | 说明 |
|------|------|
| `55aad5d` | feat: 绿色版启动器改为编译版 exe；修掉换机器回试用的指纹竞态 |
| `90f9895` | docs: 新增 DOC/ 开发者文档；把绿色版手写入口与说明纳入仓库 |
| `e46d2fb` | docs: README 增加「开发 / 自己构建」入口，指到 DOC/ |
| `640d918` | docs: 开发者文档与 v1.0.6 定稿（合并 .gitignore 规则、定稿说明、残留补记三笔；另含 DOC/BUILD-SCRIPT.md 不附带 typora-his 的更正） |
| `4fa1e73` | docs: 版本口径统一（发行版本 v1.0.6 / 基于 Typora 1.14.10）并新增 DOC/VERSIONS.md 版本改造对照 |

标签 `v1.0.6` 打在本版**最后一次提交**上（含全部代码与文档改动）；因最后一笔在定稿时被 amend 过，其哈希以 `git rev-parse v1.0.6` 为准。

---

## [1.0.5] - 2026-10-23

### 新增
- **绿色版支持：激活实体的 fingerprint 运行时动态匹配本机**
  - **背景**：把打完补丁的 Typora 整目录打包（绿色版，解压即用）时，激活实体里的 `fingerprint`（机器码 `i` 字段）是打包机器的值；Typora 本地校验会拿它比对**本机运行时指纹**，不一致即 `onUnfillLicense` 吊销回试用（deviceId 机器名|用户名不参与比对，已实验证实）。
  - **实现**（`getInsertCode` 注入模板）：
    1. 新增 `__getRealFp()`：运行时调用 Typora 内部 IPC 处理器 `license.machineCode`（`electron.ipcMain._invokeHandlers`），取本机真实 `{v,i,l}` 中的 `i`；
    2. 5 处验签/续订伪实体（publicDecrypt、privateDecrypt、net.request、protocol.handle、net.fetch）在实例化后把 `fingerprint` 覆盖为 `__getRealFp()`（取不到时回退静态值）。
  - **效果**：同一份绿色包在任何机器上首次启动即自激活，无需跑脚本、无需输入机器码邮箱。
  - **验证**（V8 Inspector 运行时取证 + 实机三连）：
    - 分离变量实验：假 fingerprint ❌ → 真 fingerprint ✅ pass（deviceId 保持假值不影响）；
    - 清备份后全新 profile 首启 `[L] pass`、二次启动 `[L] pass`；`renewLicense` 日志确认实体的 `l` 为假值仍通过（证实只比对 fingerprint）。

### 说明
- **绿色版打包要点**（v1.0.5 注入代码已含动态 fingerprint，打补丁流程不变）：
  - 可删 `Typora.exe.bak`、`resources/app.asar.bak`（回滚用，绿色包不带）；
  - **必须保留** `resources/app.bak/`（3 个文件：launch.dist.js 原版、atom.compiled.dist.jsc、package.json）——fs hook 把 `resources/app/` 的读取重定向到 `app.bak/`，删了它 Typora 启动即卡死（实测）；
  - fuse `EnableNodeCliInspectArguments` 保持默认关闭状态即可；测试用调试探针（`inspector.open`）打包前必须移除。
- 脚本激活路径（start.bat 三步）行为不变：本机跑脚本时机器码邮箱由用户输入，`__getRealFp()` 取到的本机值与输入一致（同源），互不影响。

### 改动文件
| 文件 | 改动 |
|------|------|
| `typora_crack.js` | 注入模板 +30 行左右（`__getRealFp` 助手 + 5 处 fingerprint 覆盖） |

---

## [1.0.4] - 2026-10-23

### 修复
- **拦截 Typora 自动更新，根治「第二天弹试用期剩余 X 天」（更新覆盖型）**
  - **现象**：激活后第二天 Typora 变回官方试用态，弹「试用期剩余 X 天」，期间用户并没有主动更新。
  - **根因**（本机 typora-old.log 实锤）：Typora 启动约 17 秒后自动 `checkForUpdates`，发现新版本后**不打扰用户直接下载安装**（10/11 日志：20:44:16 查询 → 20:44:23 开始下载 `typora-update-x64-1.14.10.exe` → 20:44:56 自动装完重启）。安装器把 `Typora.exe` 与 `app.asar` 换回官方新版后，Electron Fuses 复原、注入代码缺席，下次启动 RSA 验签失败即 `onUnfillLicense` 吊销授权。
  - **修复**（`getInsertCode` 注入模板内新增 `isUpdateUrl()`，4 个网络出口全部拦截，命中即返回 204 空响应，更新器解析失败静默放弃）：
    1. `globalThis.fetch`（Node 层兜底）；
    2. `electron.net.request`（electron-fetch 默认通道）；
    3. `electron.protocol.handle("https")`（渲染进程请求）；
    4. `electron.net.fetch`（主进程 fetch）。
  - **误拦自查**：单测确认 `api/client/activate`、`api/client/renew`（授权续订）不受影响；拦截范围仅限 `typora.io` / `typoraio.cn` 域下的 `/releases/`、`downloads.typoraio`、`download.typora`、`typora-update`。
  - **验证**：注入代码 `node --check` 通过；9 组 URL 判定单测通过；实机重注入后 `[L] pass`、`trailRemains is 15`，启动 29 秒后更新检查被拦（日志无 `update-available`、无 `start downloading`），授权与 12h 续订正常。

### 加固
- `process.stdout/stdin/stderr.setEncoding` 加 TTY 判断：脚本被管道/重定向运行时不再崩溃（不影响双击 `start.bat` 的正常使用）。

### 说明
- **第二类失效原因不变**：手里还是 v1.0.2 及更早发行包的同学，其注入代码里没有 2nd 二次校验屏蔽（约 20% 概率吊销授权 + IDate 写死激活日），表现为弹窗天数一天比一天少（14、13、12…）。这类必须换最新包重跑脚本，仅更新 Typora 无效。
- 若用户主动想升级 Typora：先跑脚本回滚为官方原版 → 安装新版 Typora → 再跑一次脚本重新激活。

### 改动文件
| 文件 | 改动 |
|------|------|
| `typora_crack.js` | 注入模板 +60 行左右（isUpdateUrl + 4 出口拦截）；TTY 加固 3 行 |
| `README.md` | 顶部加「务必使用最新版」提示；FAQ 重写过期条目 |

### 提交信息
```
fix: 拦截 Typora 自动更新，根治更新覆盖导致的次日授权失效（v1.0.4）
```

---

## [1.0.3] - 2026-09-10

### 修复
- **屏蔽 2nd 二次校验定时器，根治运行中弹「试用 0 天」**
  - **现象**：Typora 运行一段时间后（打开新文件的瞬间可见）弹出「试用期剩余 0 天」，重启后恢复正常。历史日志共出现 5 次（7/23、8/24、8/25、8/31、9/8）。
  - **根因**（15 个日志样本证实）：Typora 每个进程启动后固定约 530 秒触发一次 `2nd` 二次校验，校验先掷随机数——rand < 0.8（约 80%）走「读注册表 → 续订 → 通过」；rand > 0.8（约 20%）**无条件执行 `onUnfillLicense` 吊销授权，全程不读注册表**。因此 1.0.1/1.0.2 的「定时补回 SLicense」对该分支完全无效（吊销清的是进程内存态，注册表补回只保证下次启动正常）。弹窗显示「0 天」是因为注册表 IDate 固定为激活日（如 08/24/2026），超出 15 天窗口后一吊销即算出 0。
  - **修复**（3 处，均在 `getInsertCode` 注入模板内）：
    1. Hook `setTimeout`（`global.setTimeout` 与 `require('timers').setTimeout` 双覆盖）：拦截启动 120 秒内注册的第一个延迟在 520000~545000ms 之间的定时器（即 2nd 校验定时器），返回带 `_destroyed` 标记的假 timer 使其永不触发；拦截只生效一次，其余定时器全部放行。
    2. 新增 `refreshIDate()`：启动即执行一次 + 每 30 分钟检查，把注册表 `HKCU\Software\Typora\IDate` 保持为当天日期，即使吊销漏网，剩余天数也显示 15 天而非 0 天。
    3. SLicense 恢复间隔由 2 秒放宽为 5 秒（吊销分支不读注册表，2 秒一次的 reg.exe 开销无收益）。
  - **验证**：注入代码 `node --check` 语法通过；拦截逻辑单测通过（530s 拦一次、第二次放行、普通定时器不受影响、clearTimeout 假 timer 安全）；实机重注入后 Typora 运行超过 9 分 20 秒（原触发点已过），`typora.log` 无新 `2nd` 行，启动序列 `[L] pass` 正常。

### 改动文件
| 文件 | 改动 |
|------|------|
| `typora_crack.js` | +38 / -2 行 |

### 提交信息
```
Author: hu <htryone@163.com>
Date:   2026-09-10

fix: 屏蔽 2nd 二次校验定时器，根治运行中弹「试用 0 天」
（含 1.0.3 更新文档；此提交包含 CHANGELOG 自身，哈希以 `git rev-parse v1.0.3` 为准）
```

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
