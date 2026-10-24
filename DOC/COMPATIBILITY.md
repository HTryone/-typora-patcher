# 版本兼容性核查

本文记录**脚本对各 Typora 版本的适配结论，以及核查证据本身**。
和 [VERSIONS.md](VERSIONS.md) 的分工：那篇讲「本工具哪版改了什么」，这篇讲「被改造的 Typora 那一版长什么样、我们凭什么认为它能改」。

原理说明见 [TECHNICAL.md](../TECHNICAL.md)，排错判据见 [TROUBLESHOOTING.md](TROUBLESHOOTING.md)。

## 已验证版本

| Typora | Electron | 结论 |
|---|---|---|
| 1.13.7 | — | ✅ 长期验证稳定 |
| 1.14.6 | — | ✅ 已验证 |
| 1.14.10 | 42.2.0 | ✅ 已验证（需 v1.0.4 及以上脚本；v1.0.6 起绿色版内置此版） |

> ⚠️ 激活前请关掉 Typora 的「自动检查更新」。官方安装包会覆盖安装目录，把 fuses 复原、注入代码换掉。v1.0.4 起注入模板自带更新拦截，但老版本包没有。

---

## 一、1.14.10 核查详情

核查对象：本机 Typora 安装目录（releaseId `49b5981e`，内置 Electron **42.2.0**，`resources/package.json` 记录的 `version` 为 `1.14.10`）。下文路径一律以 `<Typora 安装目录>` 为根。

### 1.1 安装目录与 asar 结构

```
Typora/
├─ Typora.exe                 227,371,136 字节
├─ Typora.exe.bak             打补丁前的原始 exe（脚本生成）
└─ resources/
   ├─ app.asar                390,779 字节  ← 注入目标（补丁后改名为 app.asar.bak）
   ├─ lib.asar                8,342,504 字节  渲染层库（MathJax4 / codemirror / jimp / diagram / pdf）
   ├─ node_modules.asar       9,970,596 字节  native-reg / electron-fetch / node-machine-id / fs-extra 等 22 个包
   ├─ page-dist/ · assets/ · style/ · html/ · locales/ · packages/ · appsrc/ · updater/
   └─ window.html · conf.default.json
```

**关键点**：主进程代码全在 `app.asar` 里，`lib.asar` 与 `node_modules.asar` **不需要改动**，所以只注入 `app.asar` 就能覆盖授权逻辑。

`app.asar` 内部只有 3 个文件（直接解析 asar 头得到，未解包）：

| 文件 | 大小 | offset | SHA256（头里记录 = 实算） |
|---|---|---|---|
| `atom.compiled.dist.jsc` | 388,344 | 0 | `93c3569e…8f692` |
| `launch.dist.js` | 1,383 | 388,344 | `3ec9df88…d97b81b` |
| `package.json` | 252 | 389,727 | `e8300ff6…5ce30` |

头部 16 字节 = `4 / 792 / 788 / 781`，数据区起点 = `8 + 792 = 800`；三个文件合计 `389,979`，`800 + 389,979 = 390,779`，与文件实际大小**完全一致**；三个文件的 SHA256 也与头里 `integrity.hash` 逐一对上 —— 说明这份 `.bak` 是未被改动过的官方原件。

> 为什么能改：`launch.dist.js` 是纯 JS 文本，可直接改写；`atom.compiled.dist.jsc` 是 V8 字节码，改不动也**不需要**改 —— 授权校验调用的是 `crypto` / `fs` / `net` 这些 Node API，在更早的层面劫持即可。

### 1.2 注入点

脚本用正则 `/require\s*\([^)]*\)\s*;/` 定位插入位置。`launch.dist.js` 是压缩过的单行代码：

```js
"use strict";const fs=require("fs"),path=require("path"),vm=require("vm"),v8=require("v8"),Module=require("module");v8.setFlagsFromString("--no-lazy"),…
```

前 4 个 `require()` 后面跟的是 `,` 而不是 `;`，因此正则**首个命中**是 `require("module");`，落在第 **98** 个字符处，插入点在其后 = 第 **116** 个字符（实测该位置之前全是 ASCII，字节偏移与字符偏移一致）。

| 检查项 | 结果 |
|---|---|
| 首个命中 | `require("module");`（char 98–115） |
| 插入点 | 116（在完整 `const` 语句之后，语法合法） |
| 注入块单独 + 拼接后解析 | `new vm.Script()` 均通过 |
| 与入口文件顶层标识符冲突 | 无（v1.0.5 注入 43 个顶层名 / v1.0.6 注入 51 个，与 `fs,path,vm,v8,Module` 无重名；`fs` 复用外层作用域） |
| Hook 早于业务代码 | 注入点 116 < `require("./atom.compiled.dist.jsc")` 位置（v1.0.5 在 15806，v1.0.6 在 17918） |

实测文件尺寸（本机两处部署）：

| 部署 | 注入块版本 | 原文件 | 注入后 | 增量 |
|---|---|---|---|---|
| 脚本版（解包到 `resources/app/`） | v1.0.5（无 `__warmRealFp`） | 1,383 字节 | 17,403 字节 | +16,020 |
| 绿色版（`green-test`） | v1.0.6（含 `__warmRealFp`） | 1,383 字节 | 19,914 字节 | +18,531 |

两份的 `app.bak/launch.dist.js` 与 `app.bak/atom.compiled.dist.jsc` 的 MD5 完全相同 —— 绿色版用的就是同一份官方原件，没有二次加工。

### 1.3 Electron Fuses

在 `Typora.exe` 中定位哨兵 `dL7pKGdnNz796PbbjQWNKmHXBZaB9tsX`，**全文件仅 1 处**（偏移 188,457,008）。其后为：

| 偏移 | 值 | 含义 |
|---|---|---|
| +32 | `01` | fuse_version = V1 |
| +33 | `09` | fuse_wire_length = 9 |
| +34…+42 | `30 30 31 30 30 31 30 31 31` | 9 个开关（ASCII `'0'`/`'1'`），即 `001001011` |

逐位解读（顺序取自 `@electron/fuses` 的 `FuseV1Options`）：

| # | fuse | 原始 | 打补丁后 | 说明 |
|---|---|---|---|---|
| 0 | `RunAsNode` | 0 | 0 | |
| 1 | `EnableCookieEncryption` | 0 | 0 | |
| 2 | `EnableNodeOptionsEnvironmentVariable` | 1 | 1 | |
| 3 | `EnableNodeCliInspectArguments` | 0 | 0 | |
| 4 | `EnableEmbeddedAsarIntegrityValidation` | **0** | **0** | **未开启** —— 不会触发 asar 完整性校验 |
| 5 | `OnlyLoadAppFromAsar` | **1** | **0** | **唯一被改写的一位** |
| 6 | `LoadBrowserProcessSpecificV8Snapshot` | 0 | 0 | |
| 7 | `GrantFileProtocolExtraPrivileges` | 1 | 1 | |
| 8 | `WasmTrapHandlers` | 1 | 1 | |

两个关键结论：

1. **`EnableEmbeddedAsarIntegrityValidation = 0`** —— 这是新版 Electron 最容易踩的坑：一旦开启，解包 asar 后 Electron 会在 `Archive::Init()` 校验头哈希，失败即 `LOG(FATAL)` 直接终止进程。1.14.10 未开启，所以「解包 + 改目录」这条路可用。
2. **`OnlyLoadAppFromAsar = 1`** —— 必须翻转。翻转后 `app.asar`（已被改名）读不到 `package.json`，Electron 自动回退到 `resources/app/` 目录。

实测逐位比对：原始 exe（`Typora.exe.bak`）wire = `001001011`，打补丁后（`Typora.exe`）= `001000011`，**只有第 5 位变了**，其余 8 位一致。绿色版 exe 与脚本版 exe 的 MD5 完全相同，即同一份已翻转的产物。

> `@electron/fuses` 是纯 ESM 包（`"type": "module"`），`require()` 需要 Node ≥ 22.12（`engines` 字段就是这么写的）。本机 Node v24.14.0 实测 `require("@electron/fuses")` 正常拿到 `flipFuses` / `getCurrentFuseWire`。

### 1.4 授权链路（在 jsc 常量池里的位置）

从 `atom.compiled.dist.jsc`（388,344 字节）里按字节偏移扫到的可打印字符串，确认 1.14.10 中授权相关调用链依然存在：

| 偏移 | 字符串 | 作用 |
|---|---|---|
| 86,881 | `native-reg` | 注册表存取模块 |
| 86,905 | `Software\Typora` | 授权键 |
| 87,138 | `[WindowsLicenseLocalStore] ` | 读写 SLicense / IDate 的日志前缀 |
| 89,618 | **`publicDecrypt`** | **RSA 公钥解密 —— Hook 点** |
| 89,679 | `jsDecrypt` | 纯 JS 解密兜底 |
| 93,078 | `base64` | SLicense 字段解码 |
| 93,109 / 93,185 | `pure = ` / `pure js failed ` | 原生解密失败时回退纯 JS |
| 94,242 | `fingerprint` | 设备指纹 |
| 95,248 | `IDate` | 安装日期（试用天数来源） |
| 96,866 | `MachineGuid` | `machineId` 由 `HKLM\…\Cryptography\MachineGuid` 派生 |
| 97,594 | **`2nd `** | **二次校验日志前缀** |
| 98,691 | `equals` | 紧邻 `2nd`，即二次校验的比较逻辑 |
| 99,564 | `[watch L] hasL: ` | license 状态监听 |
| 103,878 / 103,929 | `renew` / `[renewLicense] license renewed in 12h` | 本地续订 |
| 104,684 | `[renewL]: unfill due to renew fail` | 续订失败 → 吊销 |
| 107,673 / 107,766 | `trailRemains is ` / `[L] installDate is ` | 试用剩余天数 |
| 108,382 | `[L] pass~` | 校验通过标记 |
| 121,040 / 121,257 | `license.machineCode` / `[machineCode] ` | 机器码入口 |
| 117,994 / 118,055 | **`api/client/activate`** / `[License] response code is ` | **激活接口 —— Hook 点** |
| 122,888 | `api/client/deactivate` | 注销接口 |
| 130,246 | `onUnfillLicense` | 吊销回调 |
| 293,589 / 329,179 | `checkForUpdates` / `typora-update` | 自动更新入口 |
| 335,010 / 338,108 | `electron-fetch` / `node-machine-id` | 网络与设备指纹依赖 |

结论：脚本的三类 Hook（`crypto.publicDecrypt`、`net.*` / `protocol.handle` 上的 `api/client/*`、`setTimeout` 屏蔽 `2nd`）在 1.14.10 上都有对应的真实调用点。

> 口径说明：上面是**字符串存在性**证据，能证明「1.14.10 里确实有这些授权环节」，不能单独证明「某个具体函数在第几行被调用」—— 字节码里属性访问是 `net` + `fetch` 两段字符串，不会以 `net.fetch` 连写出现。要证明调用点得用 DevTools 断点或 hook 打日志，见第五节。

### 1.5 机器码字段

**本机 1.14.10 实测**（`%APPDATA%\Typora\typora.log` 里 `[machineCode]` 那行 base64 解码，值已脱敏）：

```json
{"v":"win|1.14.10","i":"<10 位指纹>","l":"<计算机名> | <用户名> | Windows"}
```

即 1.14.10 的 `license.machineCode` 返回的仍是 **`{v, i, l}`**，`i` 就是指纹。脚本固定读 `mc.l` / `mc.i` / `mc.v` 在这个版本上**取得到值**，不会变成 `undefined`。

> 对照：有复刻者（`shadowyy/-typora-patcher` 的 `985f0d0`）记录 1.14.10 的机器码是 `{v,l,f,u}`（`f`=指纹、`u`=设备 uuid），并据此做了字段回退。他引用的样本是**激活请求体**（另一条链路），不是 `license.machineCode` 的返回值。本机复测不支持那个前提，因此我们**没有**把字段回退当作「1.14.10 修复」写进代码。若将来某版真的换了字段名，症状与处置见 [TROUBLESHOOTING.md](TROUBLESHOOTING.md)。

### 1.6 试用天数来源

日志顺序：

```
[WindowsLicenseLocalStore] IDate : 10/24/2026
[L] installDate is 10/24/2026, trail remains: 15 days
```

剩余天数取自注册表 `IDate`，不是 `profile.data` 里的 `_iD`。所以「启动即刷新 + 定时把 `IDate` 写回当天」的做法有效，清理 `profile.data` 不影响天数显示。

---

## 二、实测验证清单

激活后按顺序确认（本机 1.14.10 已逐项走过）：

1. **落盘**：`resources/` 下出现 `app/`、`app.bak/`，原 `app.asar` 变成 `app.asar.bak`；`Typora.exe.bak` 存在。
2. **注册表**：`reg query HKCU\Software\Typora` 应有

   | 值 | 期望 |
   |---|---|
   | `SLicense` | `RHJlYW1OeWE=#0#1/1/2059` |
   | `IDate` | 当天日期（`MM/DD/YYYY`） |
   | `MFp` | 本机真实指纹（v1.0.6 起才有，是指纹缓存） |

3. **日志判据**：`%APPDATA%\Typora\typora.log` 里出现
   `[WindowsLicenseLocalStore] SLicense : RHJlYW1OeWE=#0#1/1/2059`（而非空值）、`renew`、`[L] pass`。
4. **静默期**：启动后静置 **12 分钟**（覆盖约 530 秒的 `2nd` 窗口），日志不应新增 `onUnfillLicense` / `2nd`，也不弹「试用 0 天」。
5. **重启**：再启动一次，重复第 3 步。
6. **回滚**（脚本版）：重跑脚本 → 提示已激活 → 输入 `Y` → `app.asar` 恢复、`app/` 消失、注册表两项被清、Typora 回官方原版。

---

## 三、尚未验证的项（**如实记录，别当已验证**）

这一节是核查时的真实缺口，下次有人手上有时间就补掉：

| 项 | 为什么没验证 | 怎么补 |
|---|---|---|
| `2nd` 是否仍固定在约 530 秒触发 | 本机日志里 `2nd` 出现 **0 次**；最长的单次会话只持续 **191 秒**（02:55:54→02:59:05），从没跑过 530 秒 | 打补丁后开一次 Typora，**挂着别关 ≥ 10 分钟**，看日志有没有 `2nd` 行 |
| 520,000–545,000ms 这个拦截区间是否仍准确 | 同上，没跑过那么久，无从判断区间对不对 | 同上；若区间外出现 `2nd`，说明延迟变了，要按实际值调 `__hookSetTimeout` |
| 自动更新拦截是否真的生效 | 补丁是 10-23 14:01 打上的；日志里 `checkForUpdates` 共 3 次，全在 **10-23 11:08 / 11:10 / 12:46**（都在打补丁之前）。补丁后仅有的两次会话分别只持续 **6 秒**（14:28）和 **9 秒**（20:58），而实测更新检查在启动后 **13.9–29.3 秒**才发生，压根没走到那一步 | 打补丁后开一次 Typora，**保持 ≥ 60 秒**，日志里不应出现 `checkForUpdates https://typoraio.cn/releases/...` |
| 换机器自适配（`__warmRealFp`） | 本机就是构建机，烘焙指纹本来就等于本机指纹，**本机怎么测都通过**，测不出竞态 | 按 [TROUBLESHOOTING.md](TROUBLESHOOTING.md) 的「模拟换一台电脑」把指纹改假 + 删 `MFp` + 全新 profile 再测 |
| macOS | 无机器 | — |

> 顺带一个**反面**证据，别误读：脚本版（v1.0.5 注入）在 10-24 20:58 那次跑出了 `[L] pass`，但日志顺序是
> `handle license.machineCode`(261ms) → `[L] pass`(262ms) → `[machineCode]`(319ms)，
> 也就是**通过发生在机器码取回来之前**。这恰好印证 VERSIONS.md 记的 v1.0.5 缺陷：它用的是注入时烘焙的指纹，本机通过只因为**本机就是打包机**。v1.0.6 的 `__warmRealFp` 预热就是为了修这个。

---

## 四、已知风险

| 风险 | 说明 | 应对 |
|---|---|---|
| `2nd` 拦截可能误杀正常定时器 | 拦截条件是「启动 120 秒内注册的、延迟落在 520–545 秒区间的第一个定时器」，命中就换成永不触发的假 timer。如果 Typora 自己也有这个量级的正常定时器（字典下载、自动保存之类），会被一起废掉 | 观察启动后 10 分钟内的字典/自动保存是否正常；异常就收窄区间 |
| `2nd` 延迟若变化，拦截会空转 | 区间是按历史样本定的；Typora 改了延迟就拦不到 | 见第三节「怎么补」 |
| 自动更新覆盖补丁 | 官方安装包会把 exe 与 `app.asar` 换回去，fuses 复原、注入缺席，下次启动验签失败即吊销 | 激活前关掉自动检查更新；v1.0.4+ 另有网络层拦截 |
| 机器码字段再变 | 新版可能调整机器码 JSON 结构（1.14.10 本机仍是 `{v,i,l}`） | 看日志 `[machineCode]` 解出来的字段；字段名变了要同步改解析 |
| `asar` 依赖已被上游弃用 | `package-lock.json` 里 `asar@3.2.0` 带 `deprecated` 提示：官方已改名 `@electron/asar`，API 不变 | 不影响运行；将来要换就整体换包名，别只改版本 |
| Node 版本下限 | `@electron/fuses` 的 `engines` 要求 Node **≥ 22.12**（它同时是纯 ESM 包） | 包内附 Node v24.x；README 已写明装 Node.js |

---

## 五、核查方法（可复现）

本文所有结论都来自**只读**手段，没有修改真实安装。下次 Typora 升级，照这五步重跑一遍即可。

**1. asar 结构与文件内容** —— 直接解析头，不解包：

```
前 4 字节 uint32 = 4
接着 3 个 uint32  →  [pickle 头长度][header 长度][JSON 字符串长度]
之后 8 + 第二个 uint32 = 数据区起点
每个节点的 offset 相对数据区起点
```

用头的 `integrity.hash`（SHA256）与实际算出来的逐文件比对，一致才说明是官方原件。

**2. 字节码字符串** —— 在内存里扫 `atom.compiled.dist.jsc` 的可打印 ASCII 片段，按偏移排序，即可还原模块的调用链与日志文案（即 1.4 那张表）。

**3. Fuses** —— 在 exe 里找哨兵 `dL7pKGdnNz796PbbjQWNKmHXBZaB9tsX`，按 `哨兵 + fuse_version + fuse_wire_length + wire` 解码；或用 `@electron/fuses` 的 `getCurrentFuseWire()` 直接读出人类可读结果。

**4. 注入可行性** —— 用 `new vm.Script()` 只做**解析**校验，不落盘；再比对注入块的顶层标识符与原文件有无重名。

**5. 实机判据** —— 一切以 `%APPDATA%\Typora\typora.log` 为准，别看「感觉正常」。关键顺序：`[machineCode]` 必须在许可校验**之前**（见第三节的反面例子）。
