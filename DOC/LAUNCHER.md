# 绿色版启动器（START-Typora.exe）

一个 **7KB、零依赖**的小 exe：双击它 → 建/修快捷方式 → 启动 `app\Typora.exe`。
它取代了早期「一整个 `.bat` 里塞内联 PowerShell」的做法。

## 为什么不再用 .bat 内联 PowerShell

那条路把四件事同时踩了：`cmd` 的引号规则、`%` 变量展开、`& | ( )` 要转义、编码（GBK/UTF-8）；
更糟的是**失败还是静默的**——用户机「桌面没出现图标」当时就是这么跳过去的，查起来很费劲。
编译版直接用 `WScript.Shell` COM 建 `.lnk`：不经 PowerShell（不受执行策略限制）、不经 cmd（没有转义问题），出错用 MessageBox 明确弹出来。

## 文件

| 文件 | 说明 |
|---|---|
| `scripts/launcher/Program.cs` | 源码（C#，只用 .NET Framework 自带库：`System.*` + `WScript.Shell` COM + `MessageBoxW` P/Invoke） |
| `scripts/build-launcher.sh` | 构建：调系统自带 `csc.exe` → `green-test/START-Typora.exe` |
| `START-Typora.exe`（产物） | **不进 git**，随绿色包/脚本包发行 |

重编（改完源码**必须**重编）：

```bash
bash scripts/build-launcher.sh
# 找不到 csc 的话，把脚本里的 Framework64 换成 Framework：
#   C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe
```

## 行为规格（照这个改，别改语义）

1. **先校验** `app\Typora.exe` 在不在。不在 → 弹错误框、退出码 1、**不建任何快捷方式、不写标记**。
2. **首次运行标记**：`%LOCALAPPDATA%\TyporaGreen.setup-done`，三态：
   - 文件不存在 = 首次运行 → 建快捷方式；
   - 内容 `done` = 装过了 → 走第 3 条自检；
   - 内容 `removed` = 用户在 `SETUP.bat` 里选过 **3（Remove everything）** → **一处都不建**（尊重用户的删除）。
3. **每次启动自检（自愈）**：`done` 状态下检查两处快捷方式（包根 `Typora Green.lnk` 与桌面同名文件）——
   - 文件不存在 → 重建；
   - 文件在但 `TargetPath` 不是当前这个包的 `app\Typora.exe`（**整个文件夹被挪走了**）→ 重建指向；
   - 判断不了（COM 报错）就当没问题，避免每次启动都重建。
4. **桌面路径**：从注册表 `HKCU\...\Explorer\User Shell Folders` 的 `Desktop` 值取（兼容桌面被重定向到 `D:\desktop` 这类情况），取不到再退回 `Environment.SpecialFolder.DesktopDirectory`。
5. **建 .lnk**：`WScript.Shell` COM，写 `TargetPath` / `WorkingDirectory` / `IconLocation`（`<exe>,0`）。
6. **最后启动** `app\Typora.exe`，工作目录设为 `app\`。

约定：不引入任何第三方 DLL；不碰注册表其它键；`.exe` 不做代码签名（文档里已写明可能被安全软件误报，后备方案是 `START-Typora.bat`）。

## 与 `START-Typora.bat`（后备入口）的差异

`.bat` 只做「标记不存在才建快捷方式」，**没有**第 3 条自检自愈 —— 它只是 exe 被误杀时的备胎。
`SETUP.bat` 的 UNREG 分支（选 3）会往标记里写 `removed`，两个入口都认这个状态。
⚠️ `SETUP.bat` 是 **CRLF + GBK 混合编码**的历史文件：要改它只许**按字节插纯 ASCII 行**，绝不能整体重写成 UTF-8（会乱码）。

## 改完至少跑这些（本机实测矩阵）

| 场景 | 期望 |
|---|---|
| 无标记、无快捷方式 | 两处都建 + 启动程序 |
| 标记 `done`、两处都在且指向正确 | **不重建**（.lnk 的 mtime 不变）+ 启动 |
| 标记 `done`、桌面 .lnk 被删 | 自动补建 |
| 整个包目录改名 | 两处快捷方式自动改指新路径 |
| 标记 `removed` | 一处都不建，但仍启动程序 |
| 路径含中文和空格（`D:\测试 目录\...`） | 建对，target 正确 |
| 缺 `app\Typora.exe` | 弹错误框、退出码 1、不建快捷方式、不写标记 |
| 双击生成的桌面快捷方式 | 程序被拉起 |

测试小技巧：把 `app\Typora.exe` 换成一个「跑起来就往 `%TEMP%` 写个文件再退出」的假 exe，就能自动化验证「快捷方式到底有没有把程序拉起来」，不用留常驻进程。
