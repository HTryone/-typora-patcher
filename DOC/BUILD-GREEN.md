# 从零构建「免安装绿色版」

绿色版 = **一份打好补丁的完整 Typora** + **几个入口文件**，解压即用，用户不需要 Node.js、不需要跑脚本、不需要输入机器码。
本页写清怎么从一个干净的官方 Typora 造出它。

## 0. 前置

| 需要 | 说明 |
|---|---|
| Windows 10/11 | 绿色版只做 Windows |
| Node.js | 跑补丁脚本用（构建机装一次；最终用户不需要） |
| 官方 Typora 安装包 | 官网下载，任意版本。**本项目不附带 Typora 本体**，绿色包里的 `app\` 是你自己这份装好后打补丁的产物 |
| Git for Windows（bash） | 编译启动器脚本用 |
| .NET Framework 4.x | Windows 自带，提供 `C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe` |

## 1. 造母本：装官方 Typora → 打补丁

```bash
git clone <本仓库> && cd <仓库目录>
npm install                 # 或直接跑脚本，它检测到缺依赖会自己装
node typora_crack.js
```

按提示输入：

1. **Typora 安装目录**（自动检测，回车确认）
2. **机器码**：Typora → 激活 → 输入序列号 → 离线激活，把那串机器码复制过来
3. **邮箱**：随便填，格式对即可

打完补丁后，安装目录里会多出这些东西（**别急着删，下面第 3 步会告诉你留哪些**）：

| 路径 | 是什么 |
|---|---|
| `resources/app/` | 注入后的三件套（`launch.dist.js`、`atom.compiled.dist.jsc`、`package.json`） |
| `resources/app.bak/` | 解包目录的原始备份（**fs hook 读它**） |
| `resources/app.asar.bak` | 原 `app.asar` 的备份（回滚用） |
| `Typora.exe.bak` | 原 `Typora.exe` 的备份（回滚用） |

## 2. 认出「注入成功」的样子

打开 `resources\app\launch.dist.js`，开头应该是注入块（能看到 `Hook破解开始`、`__warmRealFp`），文件比原始的大一截（十几 KB）。
文件太小 / 搜不到这些标记 → 说明注入没生效，别往下走。

## 3. 绿色化：打包规则（**这一节的每一条都是实测踩出来的**）

**必须保留**

- `resources\app\`：注入后的程序要用的三件套；
- `resources\app.bak\`：fs hook 把 `resources\app\` 的读取重定向到这里，**删了 Typora 启动直接卡死**（实测）；
- `resources\` 下其它原有内容：`node_modules.asar`、`lib.asar`、`assets`、`locales`、`appsrc`、`packages`、`style`、`page-dist`、`updater`、`window.html`、`conf.default.json` 等，一个都别少。

**可以删（绿色包不带）**

- `Typora.exe.bak`、`resources\app.asar.bak`：回滚用的备份，绿色包不需要；
- 构建时的调试残留：注入模板里打开 DevTools / `inspector.open()` 之类的探针（模板里已有开关，构建时保持关闭）；
- `app\` 目录下的多余副本：例如构建过程中复制进来的 `typora_crack.js`（运行时不需要，留着还会让使用者困惑）。

**包根目录应该是**

```
green-test/                 ← 打包时压这一层（别把 green-test 这个名字套进去）
├─ app/                     ← 打好补丁的 Typora
├─ START-Typora.exe         ← 启动器（下一步编译）
├─ START-Typora.bat         ← 启动器后备（exe 被误杀时用）
├─ SETUP.bat                ← 可选设置菜单
├─ README-先看我.txt
└─ 使用说明.md
```

## 4. 编译启动器

```bash
bash scripts/build-launcher.sh      # → green-test/START-Typora.exe（约 7KB）
```

源码在 `scripts/launcher/Program.cs`，行为规格与测试点见 [LAUNCHER.md](LAUNCHER.md)。改过源码**必须重编**，包里的 exe 才会跟着变。

## 5. 打包前自查清单

- [ ] `app\resources\app.bak\` 三个文件在；
- [ ] `app\resources\` 下**没有** `app.asar` / `app.asar.bak`；
- [ ] `app\resources\app\launch.dist.js` 里有 `Hook破解开始`；
- [ ] 没有调试探针残留（DevTools/inspector）；
- [ ] `START-Typora.exe` 在，双击能建快捷方式、能起 Typora；
- [ ] 删掉本地跑出来的 `Typora Green.lnk`（那是本机快捷方式，别打进包）；
- [ ] 路径里有中文/空格也测一遍（已实测 OK）。

## 6. 压包 + 上传

- 用 7-Zip 压 `green-test\` 的**内容**，命名 `Typora.Green.7z`；
- 传到对应 Release，文件名固定 `Typora.Green.7z`（见 [RELEASE.md](RELEASE.md)）。

## 7. 为什么换电脑也能用（指纹自适配）

绿色包是把**构建机**的指纹烘焙进注入文件的，但 Typora 首次许可校验是**同步**取指纹、而机器码是**异步**查出来的。
注入模板里的 `__warmRealFp()` 会在 jsc 加载前就轮询预热，把本机真实指纹先缓存好（实测 70～80ms 就位，首次校验在 250～310ms），取到后还会写进 `HKCU\Software\Typora\MFp` 供下次启动同步命中。
所以：**同一份绿色包在任何机器上首次启动都能自激活**。

注意别把这条逻辑改坏：实体里的指纹必须等于本机指纹，否则会走 `pure = undefined → no info → onUnfillLicense` 回试用。
自测办法见 [TROUBLESHOOTING.md](TROUBLESHOOTING.md) 的「模拟换一台电脑」。
