# Typora 激活脚本 - 技术文档

## 概述

`typora_crack.js` 是一个基于 Node.js 的 Typora 激活脚本，支持 Windows 平台。通过解包 app.asar、注入 Hook 代码、修改 Electron Fuses 和写入注册表授权信息，实现对 Typora 的激活。

## 依赖

| 包名 | 版本 | 用途 |
|---|---|---|
| `asar` | latest | 解包/打包 Electron asar 归档 |
| `chalk` | 4.x | 终端彩色输出（v4 支持 CommonJS） |
| `readline-sync` | latest | 同步读取用户输入 |
| `iconv-lite` | latest | GBK→UTF-8 编码转换（注册表输出解码） |
| `@electron/fuses` | latest | 修改 Electron Fuses 配置 |

脚本首次运行时自动检测 `node_modules`，不存在则执行 `npm init -y` + `npm install`，完成后自动重启自身。

## 核心流程

```
启动 → 检测依赖 → 选择路径 → 验证路径 → 输入机器码/邮箱
  → 关闭 Typora 进程 → 解包 app.asar → 备份 app 目录
  → 备份 app.asar → 修改 Electron Fuses → 注入 Hook 代码
  → 写入注册表授权 → 完成
```

## 模块说明

### 1. 自动依赖安装（第 1-37 行）

- 检测 `node_modules/` 目录是否存在
- 不存在则自动 `npm init -y` + `npm install`
- 安装完成后 `spawnSync(process.execPath, ...)` 重启脚本

### 2. 注册表操作（第 42-87 行）

**`writeRegValue(key, name, value)`**
- 调用 `reg add` 写入 `REG_SZ` 值
- 用于写入 `SLicense` 和 `IDate`

**`deleteRegValue(key, name)`**
- 调用 `reg delete` 删除值
- 回滚时清理授权信息

### 3. 缓存机制（第 92-117 行）

- 缓存文件：`~/.typora_crack_config.json`
- 存储内容：`path`（安装路径）、`machineCode`（机器码）、`email`（邮箱）
- `readCache()` 读取时验证路径有效性（检查 `Typora.exe` 是否存在）
- `saveCache()` 写入时合并已有字段，不丢失其他缓存

### 4. 路径查找（第 122-248 行）

**查找优先级：**
1. 缓存文件 → 提示 Y/N
2. 注册表 `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\Typora.exe` → 提示 Y/N
3. PowerShell 文件夹选择对话框
4. 手动输入路径（图形界面不可用时降级）

**`readRegPath()`**
- 使用 `iconv-lite` 将 GBK 编码的 `reg query` 输出转为 UTF-8
- 正则匹配 `REG_SZ` 行，提取路径并去掉 `Typora.exe` 后缀

### 5. 机器码与邮箱输入（第 253-298 行）

- 优先使用缓存中的激活信息
- 手动输入时进行非空校验，空值则重新输入
- 输入后自动保存到缓存

### 6. 回滚还原（第 303-347 行）

`rollbackTypora(path)` 执行：
1. `taskkill /F /IM Typora.exe` 强制关闭进程
2. `app.asar.bak` → 还原为 `app.asar`
3. `Typora.exe.bak` → 还原为 `Typora.exe`
4. 删除解包的 `app/` 和 `app.bak/` 目录
5. 删除注册表 `SLicense` 和 `IDate`

### 7. 已激活检测（第 352-393 行）

- `app.asar` 不存在 + `app/` 存在 → 判定为已激活，提示是否回滚
- `app.asar` 不存在 + `app/` 不存在 → 判定为安装目录损坏

### 8. Hook 注入（第 398-465 行）

`getInsertCode()` 生成的注入代码执行以下操作：

1. **DevTools 控制**：`EnableHookDebug=true` 时禁用 `app.quit()` 并自动打开开发者工具
2. **文件系统 Hook**：拦截 `fs.readFileSync`、`fs.readFile`、`fs.stat` 等方法，将 `resources\app\` 路径重定向到 `resources\app.bak\`，使 Typora 读取原始未修改的文件
3. **fs.promises Hook**：同上，拦截 Promise 版本的文件操作
4. **crypto.publicDecrypt Hook**：替换 RSA 解密结果，返回伪造的授权 JSON
5. **协议拦截**：拦截 `https` 协议，对 `api/client/activate` 和 `api/client/renew` 请求返回伪造的成功响应

**注入位置**：`launch.dist.js` 中第一个 `require(...);` 语句之后

### 9. Electron Fuses 修改（第 538-542 行）

```javascript
await flipFuses(TyporaEXE, {
    version: FuseVersion.V1,
    [FuseV1Options.OnlyLoadAppFromAsar]: false,
});
```

将 `OnlyLoadAppFromAsar` 设为 `false`，允许 Electron 从解包的 `app/` 目录加载应用，而非仅从 `app.asar` 加载。

### 10. 注册表授权写入（第 555-556 行）

```
HKCU\Software\Typora\SLicense = "RHJlYW1OeWE=#0#1/1/2059"
HKCU\Software\Typora\IDate = "{当前日期 MM/DD/YYYY}"
```

## 文件变更清单

激活后 Typora 安装目录变更：

| 文件 | 操作 |
|---|---|
| `Typora.exe` | 修改 Fuses |
| `Typora.exe.bak` | 新建（原始 exe 备份） |
| `resources/app.asar` | 重命名为 `app.asar.bak` |
| `resources/app.asar.bak` | 新建（原始 asar 备份） |
| `resources/app/` | 新建（解包目录，含注入代码） |
| `resources/app.bak/` | 新建（解包目录原始备份） |

## 启动方式

- **命令行**：`node typora_crack.js`
- **双击**：`start.bat`（自动检测 Node.js）

## 前置条件

- Windows 系统
- Node.js 已安装
- Typora 已安装
