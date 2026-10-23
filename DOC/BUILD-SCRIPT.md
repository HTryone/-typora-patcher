# 配一个「脚本版」发行包

脚本版 = 让用户**在自己机器上**跑补丁：包里只有 Node.js 运行时、补丁脚本和说明；Typora 由用户自己装。
适合「想自己动手 / 不放心用别人打好的包 / 需要自定义安装路径」的场景。

## 包里的 5 项（缺一不可，**就这 5 项**）

| 文件/目录 | 从哪来 | 说明 |
|---|---|---|
| `start.bat` | 仓库根 | 用户双击的入口（自动找/装 Node，再跑补丁） |
| `typora_crack.js` | 仓库根 | 补丁主脚本 |
| `发行说明.txt` | 当次新写 | 包内说明（见下面「写法约定」） |
| `node.js/` | 本地母本 | Node.js 安装包（`.msi` 推荐 + `.zip` 便携备用） |
| `node_modules/` | 仓库根 | 脚本依赖（约 3.3MB / 318 个文件，用户首次运行免联网） |

> ⚠️ **不附带 Typora 安装包。** 自 v1.0.5 起包里不再有 `typora-his/`：Typora 是官方版权物，用户自己去官网下载任意版本即可，没必要重复分发（还会把包体从 70MB 撑到 260MB+）。压缩完看大小，**约 70MB 才对**。

## 目录与命名

```
typora-patcher-<版本号>/     ← 压这一层（压出来解压后应直接看到 start.bat）
├─ start.bat
├─ typora_crack.js
├─ 发行说明.txt
├─ node.js/
└─ node_modules/
```

- 压缩包命名：`typora-patcher-<版本号>.7z`（与 Release 资产名一致）。
- Node.js 安装包（约 67MB）**用 NTFS 硬链接复用母本**，别复制，省空间也省时间：

```bash
# Git Bash 里（目标目录必须先建好，否则 ln 可能静默失败）
mkdir -p fax/typora-patcher-<版本号>/node.js
ln fax/node.js/<文件名> fax/typora-patcher-<版本号>/node.js/<文件名>
# 或 cmd：
mklink /H "D:\...\fax\typora-patcher-<版本号>\node.js\<文件名>" "D:\...\fax\node.js\<文件名>"
```

（硬链接要求同一分区；链接数 ≥2 是正常的。）

## 写法约定（**踩过坑**）

- `发行说明.txt`：**UTF-8 带 BOM + CRLF**（老记事本打开不乱码、换行正常）；一级标题写 `typora-patcher v<版本号>`，**别出现「激活/破解/补丁工具」这类显眼字样**。
- `【包内内容】` 段落只列上面那 5 项，**不要出现 `typora-his`**。
- `.bat`：**纯 ASCII**，不写中文（编码一错就乱码）；需要中文提示就交给 Node 脚本输出。
- `node_modules/`：打进去，用户首次运行就免联网（脚本检测到缺依赖才会去 `npm install`）。
- `package.json`：不用塞进发行包。

## 打包前

1. **先看进程**：确认没有 Typora / Node 在占用包内文件；
2. 核对 5 项齐全：`ls -1` 应输出 `node.js node_modules start.bat typora_crack.js 发行说明.txt`；
3. 确认**没有** `typora-his/`；
4. 本地解压自测一遍：解压 → 双击 `start.bat` → 按提示走完 → Typora 打开是激活态；
5. 再上传（见 [RELEASE.md](RELEASE.md)）。

## 用户侧流程（写说明时照这个口径）

1. 装 Node.js（包内 `node.js/node-vXX.msi`，已装可跳过）；
2. 官网装 Typora，打开一次再关闭；
3. 双击 `start.bat` → 选安装目录 → 粘贴机器码 → 填邮箱；
4. 打开 Typora 即为激活态。想还原：再跑一次脚本，检测到已激活时输入 `Y`。
