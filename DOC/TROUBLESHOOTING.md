# 排错手册（Windows）

## 日志在哪

```
%APPDATA%\Typora\typora.log        ← 当前日志
%APPDATA%\Typora\typora-old.log    ← 滚动后的旧日志
```

## 一句话判据

- 搜到 **`[L] pass`** → 授权有效；
- 搜到 **`no info`** 紧跟 **`onUnfillLicense`** → 本地校验没过，回试用倒计时。

## 许可相关关键行（按出现顺序读）

| 日志行 | 含义 |
|---|---|
| `[WindowsLicenseLocalStore] SLicense : <值>` | 从注册表读到的授权串（空 = 被清过） |
| `handle license.machineCode` | 开始取本机机器码 |
| `[machineCode] <base64>` | 取到了，括号里那串 base64 解出来就是 `{"v":...,"i":指纹,"l":设备}` |
| `pure = undefined` | 本地纯 JS 验签没过（**正常现象**，这条路本来就不走通） |
| `renew` → `[renewLicense] license renewed in 12h` → `[L] pass` | 走续订路径，**通过** |
| `no info` → `onUnfillLicense` | 没拿到有效授权信息 → 回试用 |
| `trailRemains is N` / `[L] installDate is <日期>, trail remains: N days` | 试用剩余天数（按 `IDate` 算） |
| `2nd 0.xxxx` | 启动约 8 分 50 秒时的概率性二次校验（约 20% 概率无条件吊销） |

**时间顺序很关键**：`[machineCode] <值>` 必须出现在 `SLicense` / `pure` **之前**。
若它出现在 `no info` **之后**，说明本机指纹没赶上那次校验 —— 这是早期版本「绿色版换电脑必回试用」的根因（机器码是异步查的、校验是同步取指纹的）。带 `__warmRealFp` 预热的最新注入模板已修掉；确认办法：`resources\app\launch.dist.js` 里搜得到 `__warmRealFp`。

## 常见现象 → 根因 → 处置

| 现象 | 根因 | 处置 |
|---|---|---|
| 第一天正常，第二天弹「试用期剩余 X 天」；安装目录里 `resources\app.asar` **又出现了** | Typora 静默自动更新，把打补丁的 exe 与 app 整个换回官方版 | 用拦截了自动更新的版本重打补丁；或换新包 |
| 运行中（约 8 分 50 秒）弹「试用 0 天」，重启又正常 | `2nd` 二次校验抽到「无条件吊销」分支 | 用拦截了该定时器的版本重打补丁 |
| 绿色版换一台电脑就未激活（本机怎么试都正常） | 打包机指纹与目标机不同，且指纹没赶上校验 | 用带预热逻辑的最新包 |
| 绿色版 `app\resources\` 下**没有** `app.asar` / `app.asar.bak` | **正常**：绿色版就是解包目录 `app\` + 原始备份 `app.bak\` | 不是故障，别照「脚本版」的经验找 asar |
| 双击 `START-Typora.exe` 没建快捷方式 | 首次运行标记已存在（之前装过）；或标记是 `removed`（选过 SETUP 的 3） | 最新启动器会自检自愈；也可 `SETUP.bat` 选 **1**；选过 3 的话它不会自动建回来 |
| `START-Typora.exe` 被杀软删了/报毒 | 未签名的自制程序常被误报 | 加白名单（包内不再提供 .bat 后备入口） |
| 双击 .md 没反应 / 每次弹选择框 | 没注册文件关联，或没勾「始终使用」 | `SETUP.bat` 选 **2**，弹窗里勾上「始终使用此应用」 |
| Typora 启动直接卡死（转圈不动） | `resources\app.bak\` 被删了（fs hook 读的就是它） | 重新构建绿色包，别删 `app.bak\` |

## 怎么确认补丁真的生效了

1. 打开 `resources\app\launch.dist.js`，开头应能看到 `/** Hook破解开始 */` 与 `__warmRealFp`（后者是最新指纹预热逻辑）；
2. 「关于 Typora」里的版本号不应该比你装的版本更高（变高了 = 被自动更新洗过）；
3. 启动后看日志：出现 `[L] pass` 即为通过。

## 模拟「换一台电脑」自测（开发者）

绿色包的指纹是构建机的，本机测永远正常，所以要**人为把指纹改坏**才能验证自适配逻辑：

1. 打开 `resources\app\launch.dist.js`，把 5 处 `"fingerprint":"..."` 的值临时改成任意假值（如 `AAAAAAAAAA`），`"deviceId"` 也可以一起改假；
2. 删掉注册表缓存：`HKCU\Software\Typora` 下的 `MFp` 值（有就删）——它是指纹缓存；
3. 用**全新配置目录**启动，避免旧 profile 干扰：

```bat
app\Typora.exe --user-data-dir=D:\tmp\fresh-profile
```

4. 看新建的 `typora.log`：出现 `[L] pass`（且 `[machineCode] <真指纹>` 排在许可校验之前）就说明自适配没问题；
5. 测完把指纹改回真值（或直接重跑补丁脚本重新注入）。

## 回滚（脚本版）

再跑一次 `start.bat` → 脚本检测到「已激活」→ 输入 `Y` → 自动还原：恢复 `app.asar` / `Typora.exe`，删掉 `app\`、`app.bak\`，并清掉注册表里的授权值。

> 绿色版没有「回滚」这个概念：想换版本就 `SETUP.bat` 选 3 清理 → 删文件夹 → 解压新包。
