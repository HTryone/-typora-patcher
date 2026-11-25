# 打包压缩规范（7z 参数与资产命名）

> 本页规定**发行二进制**的压缩参数、文件命名与产物落位。发版整体流程见 [RELEASE.md](RELEASE.md)；绿色版怎么组包见 [BUILD-GREEN.md](BUILD-GREEN.md)。
> 所有参数都是 2026-11-25 实测对比过的（证据见文内），别凭感觉换参数。7-Zip 版本 26.06（`D:\Apps\7-Zip\7z.exe`）。

## 一、压缩参数（固定，别改）

### 绿色版 / 脚本版 .7z 包

```powershell
# 7-Zip 26.06 实测环境：D:\Apps\7-Zip\7z.exe（本机注册表指向）
& 'D:\Apps\7-Zip\7z.exe' a -t7z -mx=9 <输出.7z> <打包目录>\* -y
```

等价 bash 写法（Git Bash）：

```bash
"/d/Apps/7-Zip/7z.exe" a -t7z -mx=9 <输出.7z> <打包目录>/* -y
```

| 参数 | 值 | 为什么 |
|---|---|---|
| `-t7z` | 7z 格式 | 与 Release 资产约定一致；解压端 7-Zip/Bandizip/WinRAR 都认 |
| `-mx=9` | 最高级别 | **实测甜点**，见下表 |
| （隐式）BCJ2 | mx=9 自动挂 | 对 exe/dll 吃掉可执行文件结构冗余，不用显式写 `-m1=BCJ2` |
| （隐式）`-ms=on` | 固实压缩 | 默认已开，不必显式写 |
| `-y` | 全部确认 | 脚本化需要，避免交互卡住 |

**实测证据**（单压 216.8MB 的 `Typora.exe`，7-Zip 26.06 x64）：

| 参数 | 结果 | 结论 |
|---|---|---|
| `-t7z -mx=9`（LZMA2:28 + BCJ2） | **55.6 MB** ✅ | 最优 |
| `-m0=LZMA:d=256m:fb=273 -m1=BCJ2` | 64.2 MB | 负优化 |
| `-m0=LZMA2:d=1536m:fb=273 -m1=BCJ2` | 64.3 MB | 负优化，还多花 2.5 分钟 |

加大字典/提高 fb 都是负优化：Electron 应用的高熵二进制（机器码+Chromium 资源）接近不可压上限，**绿色版整包 ≈ 94 MB 就是下限**，别再花时间试。

### 组包用的文件复制（压包前）

```powershell
# 本机 Typora → 打包目录：排除回滚备份与卸载器（.gitignore 同思路）
robocopy D:\Apps\Typora D:\AI\typoraj\tmp\green-local\app /E `
  /XF Typora.exe.bak app.asar.bak unins000.dat unins000.exe `
  /XD app.asar.unpacked
# robocopy 退出码 0-7 都是成功，8+ 才是失败
```

### 解压核对（压完自查用）

```powershell
& 'D:\Apps\7-Zip\7z.exe' x <包>.7z -o<核对目录> -y
& 'D:\Apps\7-Zip\7z.exe' l <包>.7z        # 列表确认无 lnk / .bak / unins000
```

## 二、产物命名与落位

> **规则：`dist\` 就是发行目录，里面一切按 Release 资产标准命名，不加本地后缀、不带临时名。**
> 用户明确约定：dist 里放的就是准备上架的东西，命名与 Release 资产一一对应。

### 本地构建产物 → `dist\`（gitignore，不进仓库）

| 产物 | dist 文件名（=上架资产名） | 来源 |
|---|---|---|
| 绿色版离线包 | `Typora.Green.7z`（**固定名，不带版本号**） | `tmp\green-local\*` 压出 |
| Go 版工具 | `typoratool-vX.Y.Z.exe`（带发行号，单 exe 不打包） | `typoraGO\build.bat` 编译，移入 dist 并按当次发行号命名 |

> **脚本版不再单独发行**：不进 dist、不上 Release。自用/私下分发时在 `script-release\` 组装压包（流程照 [BUILD-SCRIPT.md](BUILD-SCRIPT.md)），参数同下表。

- 版本信息住在 **tag 与 Release 说明**里，不编码进绿色版文件名（下载脚本/教程链接可长期写死 URL）；
- Typora 实际内置版本的核对靠包内 `README-先看我.txt`，不靠文件名；
- dist 里**不放**解压核对目录——核对完删掉，别让测试残留混进发行物；
- 发新版本时直接覆盖同名资产（`Typora.Green.7z` 永远指向最新绿色版）。

### Release 上架资产名（固定，用户脚本依赖）

| 资产 | 命名 |
|---|---|
| 绿色版 | `Typora.Green.7z`（**不带版本号**，固定名） |
| Go 工具 | `typoratool-vX.Y.Z.exe`（单 exe 不打包） |

`dist\` 里的名字就是上表的名字，**上传时零改名**——dist 即发行目录。脚本版不是资产，不出现。

### 包内清点（打包前自查，每次都过一遍）

- [ ] 无 `Typora Green.lnk`（启动器自愈会在本机测试时重建，**每次打包前删**）
- [ ] 无 `Typora.exe.bak` / `resources\app.asar.bak` / `unins000.*`（回滚备份与卸载器不进包）
- [ ] 包根四件齐：`app\`、`START-Typora.exe`、`SETUP.bat`、`README-先看我.txt`、`使用说明.md`
- [ ] `START-Typora.exe` 是**刚重编**的（改过 `Program.cs` 必须重编，见 LAUNCHER.md）且带图标（右键属性看图标是 T 字；文件应约 474KB，无图标版本只有 8KB）
- [ ] 包内说明文档里的 Typora 版本号与实际内置版本一致

## 三、重编启动器（含图标）

```bash
bash scripts/build-launcher.sh
```

脚本自动带 `-win32icon`（取 `tmp/green-local/app/resources/assets/app.ico`，回退本机 `D:\Apps\Typora\resources\assets\app.ico`；两处都没有才无图标编译）。改过 [scripts/launcher/Program.cs](../scripts/launcher/Program.cs) **必须重编**。

## 四、本次实测留下的参数红线

- **`dist\` 即发行目录**：只放按资产标准命名的最终产物；用户解压核对后的目录要删掉，不留混名文件夹（如旧 `-local` 命名产物）。
- 打包源永远是 `tmp\green-local\`；解压目录只用于人工核对，别把它压回包（会混入测试残留）。
- 压缩前确认 Typora 没在运行（进程占用会让 robocopy/7z 产出不一致的中间态）。
