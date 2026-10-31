<p align="center">
  <img src="assets/typoratool-logo.png" width="112" alt="typoratool 图标"/><br/>
  <b>typoratool</b>
</p>

# typoratool — Typora 激活工具（Go 版）

`typora_crack.js` 的 Go 重写版：**单文件 exe、零第三方依赖、不装 Node 也能跑**。
默认是**图形界面**（原生 Win32，无第三方 UI 库）；控制台菜单用 `--cli`。

## 使用（真·一键）

双击 `typoratool.exe` → UAC 点「是」→ 窗口里点 **「一键激活」**，结束。**机器码、邮箱全都不用填。**

- **机器码自动生成**——本地校验只比对 fingerprint，而 fingerprint 在运行时由注入代码取本机真实值
  （优先用注册表里缓存的上次指纹，首次激活用占位符），deviceId 不参与比对；
- **邮箱自动生成**（`<用户名>@local.device`，仅写进激活实体用于界面展示）；
- 安装目录自动检测（注册表 App Paths → 常见路径）；
- 激活完成后自动启动 Typora。

窗口四个按钮：`一键激活` / `回滚还原` / `刷新状态` / `使用教程`（教程窗口里写了机器码怎么找、邮箱怎么写、常见问题）。
另外：**重复启动会被挡住**（单实例锁，第二次双击只会把已有窗口唤到前台）。

## 教程：机器码怎么找？邮箱怎么写？路径怎么选？

**机器码**是 Typora 按你电脑硬件生成的一段编号，官方用它签发许可证。
位置：Typora → 菜单「帮助」→「我的许可证」→ 点「离线激活」，能看到一串 Base64 样子的长字符（可能带 `+ / =`）。

> 本工具**不需要**你手动粘贴它——它会自动生成并让注入代码在运行时校准到本机真实指纹。
> 只有想指定某个特定机器码时才需要手动填（见下面命令行参数）。

**邮箱**随便写：它只是激活实体里的一个显示字段，Typora 不会真发邮件。工具默认生成 `<用户名>@local.device`。

**安装路径**要选「装了 Typora 的那个根文件夹」——里面能直接看到 `Typora.exe` 的那一层：

| 你选的 | 结果 |
|---|---|
| `D:\Apps\Typora` | ✅ 正解 |
| `D:\Apps\Typora\Typora.exe` | ⚠️ 选到文件了 → 自动纠正成上一层 |
| `D:\Apps\Typora\resources` | ⚠️ 选深一层 → 自动纠正 |
| `D:\Apps\Typora\resources\app` | ⚠️ 更深 → 也会自动纠正（最多往上找三层） |

一般不用管：工具按 ① 上次记住的路径 → ② 注册表 `App Paths`（HKLM 再 HKCU）→ ③ 常见安装位置
的顺序自动检测。**官方安装包装的 Typora，装在哪个盘都能自动找到**；绿色版 / 免安装版 / 装在非常规
目录时自动检测不到，点界面上的「浏览…」自己选一次即可（会记住，下次自动带出来）。

**常见问题**

| 情况 | 处理 |
|---|---|
| 激活后仍显示试用 | 关掉 Typora 重开（补丁在启动时生效）；仍有问题先「回滚还原」再「一键激活」 |
| 每次打开都弹未激活 | 他机旧试用残留导致；点「回滚还原」→ 再「一键激活」 |
| 想升级 Typora | 补丁已拦截自动更新：先「回滚还原」→ 装新版 → 再「一键激活」 |
| 杀毒软件报毒 | 未签名 exe 常见误报，加白名单 |
| 想恢复原版 | 点「回滚还原」，备份自动还原、不留残留 |

## 命令行（可脚本化，零输入）

```
typoratool.exe --status                     只读查看状态
typoratool.exe --activate --yes             直接激活（全自动，零输入）
typoratool.exe --rollback --yes             直接回滚
typoratool.exe --dir "D:\Apps\Typora"       指定安装目录
typoratool.exe --machine <Base64> --email x@y.z   可选：手动指定
typoratool.exe --cli                        用控制台菜单（数字选择）
typoratool.exe --no-elevate                 调试用：不自动提权
typoratool.exe --no-color                   控制台模式关闭彩色输出
```

缓存位置 `%USERPROFILE%\.typoratool.json`；与 JS 版 `~/.typora_crack_config.json` 互不干扰；
补丁产物与 JS 版完全同构，**两套工具可互相回滚**。

## 目录结构（按职责分包）

```
typoraGO/
├── build.bat            双击编译（产物 typoratool.exe，GUI 子系统、不弹 CMD 窗口）
├── go.mod               module typoratool
├── typoratool.exe       编译产物（已 gitignore）
├── assets/
│   ├── typoratool.ico            程序图标（9 个尺寸，从 Typora 原版 T 图标重绘）
│   ├── typoratool.exe.manifest   只声明 DPI 感知 + asInvoker
│   └── *-preview-*.png           图标预览（白底/黑底各一张，生成时自动产出）
├── tools/
│   └── make_icon.py     重新生成图标（需 Pillow；源图取 Typora 的 icon_512x512@2x.png）
└── src/
    ├── main.go          入口：命令行参数、提权、单实例锁、界面分发
    ├── rsrc.syso        图标 + 清单编译成的资源对象（由 build.bat 自动生成/复用）
    ├── ui/              原生 Win32 控件封装（窗口/DPI 缩放/控件/消息/文件夹选择框）
    ├── console/         控制台界面（--cli 用）
    ├── system/          Windows 系统层：注册表、进程、提权
    ├── asar/            asar 归档解析与解包（只读）
    ├── fuses/           Electron 保险丝读写
    └── patcher/         补丁核心：实体/机器码/注入模板/激活/回滚/配置
```

依赖方向单向：`main → ui / console / patcher / fuses / system`，`patcher → asar / fuses / system`，无循环依赖。

## 图标

沿用 Typora 原来的「白色圆角卡片 + 黑色 T」，但**按尺寸重绘**（原图直接缩放时小尺寸会糊）：

| 尺寸 | 处理 |
|---|---|
| 16 / 20 / 24 px | 去掉背景灰线、T 放大到 62%、不加描边（保证任务栏/标题栏能认出是 T） |
| 32 / 40 / 48 px | T 占 57%、灰线极淡、细描边 |
| 64 px 以上 | T 占 55%、还原原图的 5 条灰线 |

所有尺寸统一在**右下角加绿色对勾徽章**（`#16A34A` 绿圆 + 白勾 + 白描边圈）——
这是和原版 Typora 区分的关键标记：任务栏里一眼能认出这是激活过的那个，不会跟官方版混淆。

重绘脚本 `tools/make_icon.py`，源图是 Typora 自带的 `resources\assets\icon\icon_512x512@2x.png`（1024×1024）。
窗口图标不是靠资源 ID 取的，而是运行时用 `ExtractIconExW` 按索引从 exe 里取（免得依赖资源 ID 编号）。

## 与 JS 版的差异（有测试佐证）

| 项 | JS 版 | Go 版 |
|---|---|---|
| 依赖 | Node 22 + npm 五个包 | 无（单 exe，syscall 直调） |
| 界面 | 控制台问答 | 原生图形界面（DPI 感知、随缩放清晰）+ `--cli` 控制台 |
| 指纹注册表缓存（v1.0.6 `__FP_KEY`） | **从未生效**（模板字面量把 `HKCU\\Software\\Typora` 转义成 `HKCUSoftwareTypora`，MFp 匹配正则同病）——一直靠 8ms 轮询兜底 | 正常生效（路径与正则均正确，`parity_test` 断言） |
| 二次激活时 exe 备份 | 每次覆盖 `.bak`（可能把已翻保险丝的 exe 存进备份） | 仅在 `.bak` 不存在时备份，回滚恢复的始终是原版 |
| 半途失败 | 可能留下中间态 | 已完成的破坏性步骤自动逆序撤销 |
| 重复启动 | 可开多个 | 单实例锁：第二次只唤前台 |

注入代码本体与 JS 版**逐字节一致**（`TestHookMatchesJSScript` 从仓库 JS 实时提取 `getInsertCode` 输出比对，
上述已知转义缺陷按 Go 版正确写法归一化）。

## 开发

```
go install github.com/akavel/rsrc@latest      # 一次性：图标/清单嵌入工具（已装则在 %USERPROFILE%\go\bin）
go test ./...                                 # 10 个测试：asar 往返/真实归档解包、保险丝翻转、
                                              # 机器码解码与自动生成、路径容错、注入点定位、代码生成、
                                              # 与 JS 版逐字节对齐、假安装端到端（激活→校验→回滚→校验）
bash build.bat 或双击 build.bat                # 编译（缺 src\rsrc.syso 会自动生成）
go build -o typoratool.exe ./src              # 手动编译（需 src\rsrc.syso 已存在，否则没图标）
python tools/make_icon.py <源图> assets/typoratool.ico    # 重新生成图标（需 Pillow）
```

- `src/rsrc.syso`：图标 + 清单的资源对象，Go 构建时自动链入（同目录有 .go 就会被识别）
- 环境变量 `TYPORATOOL_TEST=1`：跳过真实注册表与 `%APPDATA%\Typora` 的写入（自测用）
- 环境变量 `TYPORATOOL_DEBUG=1`：把启动阶段进度写 `%TEMP%\typoratool-debug.log`（排查卡死用）

## 免责声明

仅供学习与技术研究，请支持正版 Typora。
