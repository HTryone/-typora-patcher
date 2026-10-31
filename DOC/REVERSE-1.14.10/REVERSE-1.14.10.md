# Typora 1.14.10 授权链路逆向报告

> 分析对象：`typora-patcher v1.0.6` 所依赖的 Typora 1.14.10（Windows x64）授权校验机制
> 分析方式：只读勘察——asar 归档解析、V8 字节码字符串表提取、PE 结构分析、注册表现状核对。**未修改任何发行文件。**
> 分析日期：2026-11-01
> 工具：Node.js（自写扫描脚本）、7-Zip 无关、无第三方逆向工具

---

## 一、分析对象与文件布局

| 文件 | 大小 | 角色 |
|---|---|---|
| `resources/app.asar` | 390,779 B | 主归档（官方原件；本机 `app.asar.bak` 的 SHA256 与头内 `integrity.hash` 逐一对上，证明未被篡改） |
| `resources/app.asar.unpacked/main.node` | 1,102,720 B | 原生 NAPI 模块（**与验证无关**，见 §5） |
| `resources/node_modules.asar` | 9,970,596 B | 依赖包（`native-reg` / `node-machine-id` 等在此） |

**app.asar 内部仅 3 个文件**（头解析：`4 / 792 / 788 / 781`，数据区起点 = 8 + 792 = 800）：

| 文件 | 大小 | SHA256（前 16 位） |
|---|---|---|
| `atom.compiled.dist.jsc` | 388,344 | `93c3569e6229acfde76167a877b69e5c`… |
| `launch.dist.js` | 1,383 | `3ec9df885d96feaa030b2e34f02007a1b`… |
| `package.json` | 252 | — |

三个文件合计 389,979 字节，800 + 389,979 = 390,779，与实际大小**完全一致**——归档无填充、无附加数据。

---

## 二、加载器 `launch.dist.js`（1,383 字节，全文见附录 A）

关键逻辑：

```js
v8.setFlagsFromString("--no-lazy");
v8.setFlagsFromString("--no-flush-bytecode");
Module._extensions[".jsc"] = Module._extensions[".cjsc"] = function (e, t) {
    let a = fs.readFileSync(t);
    setFlagHashHeader(a);          // 用空脚本的 hash 头覆盖 jsc 的 hash 头
    let r = buffer2Number(getSourceHashHeader(a));
    // r>1 时构造对应长度的全零宽空格字符串作为"源码占位"，使 cachedData 的源哈希匹配
    let c = new vm.Script(o, { filename: t, cachedData: a });
    if (c.cachedDataRejected) throw Error("Invalid or incompatible cached data");
    ...
    return s.apply(e.exports, d);  // 在当前模块上下文里执行字节码
};
require("./atom.compiled.dist.jsc");
```

要点：
- `jsc` 是 **V8 code cache**（`magic = c306dec0112c8476`），不是可读 JS；
- 它**只能在编译它的那个 V8 里运行**——本机 Node 24.21.0 实测加载报 `cachedDataRejected: true`，证实跨 V8 不可用，也就**没有通用的 jsc 反编译工具**；
- `launch.dist.js` 是**纯文本 JS 且是最早执行的入口**，这就是所有补丁都选它做注入点的原因。

---

## 三、授权链路全景（jsc 字符串表 1,859 条还原）

### 3.1 主流程骨架

```
应用启动
 ├─ 读 InstallDate（优先 profile.data 的 birthtime，损坏则重写）
 ├─ native-reg 读注册表 Software\Typora → SLicense / IDate
 │    SLicense 格式 = [base64 段]#[计数器]#[有效期]    例: DreamNya#0#1/1/2059
 ├─ [watch L] 观察者：hasLicense 状态变化 → 续期或弹激活窗
 │    ├─ 有证 → 12h 周期向 typoraio.cn/store（大陆镜像）续期
 │    │        失败 → "[renewL]: unfill due to renew fail"（清空 SLicense）
 │    └─ 无证 → 弹 page-dist/license.html?ecp=...&dayRemains=15
 ├─ 验签：crypto.publicDecrypt（Node 内置 API，jsc 只负责调用）
 │    └─ 双实现：publicDecrypt + jsDecrypt（Typora 自写的纯 JS RSA）
 │       失败回退日志："pure js failed"
 ├─ 指纹采集：
 │    · HKLM\SOFTWARE\Microsoft\Cryptography → MachineGuid（native-reg 读）
 │    · username（Windows: COMPUTERNAME / macOS: scutil --get ComputerName / Linux: hostname）
 │    · 组合出 deviceId 与 fingerprint
 └─ 服务器交互（store.typora.io / typoraio.cn）
      · 激活：POST api/client/activate（机器码 + 邮箱 + license code）
      · 状态码：OUT_OF_LIMIT（超设备数）/ INVALIDATE / WRONG_USER（邮箱不匹配）
      · 离线激活入口：offlineActivation（本版仍存在）
```

### 3.2 机器码（`license.machineCode`）

- `handle license.machineCode` 是主进程注册的 IPC 处理器；
- 1.14.10 实测返回 base64(`{"v":"win|1.14.10","i":"rxBwL7id6u","l":"DESKTOP-4DC7V5C | Htryone | Windows"}`)：
  - `i` = fingerprint（硬件指纹，参与本地比对）
  - `l` = deviceId（人读的设备名，**本地校验不比对**——分离变量实验证实）
- 有复刻者声称 1.14.10 改成了 `{v,l,f,u}`；本机复测**不支持**该说法，`{v,i,l}` 仍有效（详见 DOC/COMPATIBILITY.md §1.5）。

### 3.3 license code 的本地校验（三层）

```
正则：^([A-Z0-9]{6}-){3}[A-Z0-9]{6}$      ← 24 位，4 段
字母表：L23456789ABCDEFGHJKMNPQRSTUVWXYZ    ← 去掉 I/O/0/1 防混淆
校验和：checksum failed                      ← 算法在字节码里，未反编译
```

三层都过才发 `api/client/activate`。服务器返回码：`OUT_OF_LIMIT`（超设备数，可选踢掉最旧设备）、`WRONG_USER`（邮箱不匹配）。

### 3.4 验签密钥的存放

- PEM/DER/裸模数特征全量扫描（`-----BEGIN`、`MII`、`30 82 01 22`、`30 81 9f`、`02 82 01 00` 等）：**0 命中**；
- `createPublicKey` / `setPublicKey` / `PUBLIC KEY` 等构造痕迹：**0 命中**；
- 结论：**公钥不以明文存在于任何发行文件**，藏在 V8 字节码常量池（编译期混淆/分段存储），或运行时构造。
- 推论：**"自己签一张合法许可证"这条路是死的**——公钥都提不出来，私钥更不可能。这正是所有补丁（含上游 DreamNya 项目）都选择 Hook 验签函数本身、而不是伪造签名的根本原因。

---

## 四、"不用机器码"的方案为什么成立（对应 v1.0.6+ 的实现）

1. **本地校验只比对 fingerprint**（`i` 字段），deviceId（`l`）不参与——分离变量实验证实；
2. fingerprint 在运行时由注入代码调用 Typora 自己的 `license.machineCode` 接口取**本机真实值**，并缓存到注册表 `HKCU\Software\Typora\MFp` 供后续启动同步命中；
3. 因此**烘焙进激活实体的 fingerprint 是什么根本不重要**——机器码整串都成了占位符，工具全自动生成。
4. `publicDecrypt` 被 Hook 后不真验签，直接返回构造的实体（`typoraGO/src/patcher/hook.go` L186-190）：

```js
crypto.publicDecrypt = function (key, buffer) {
    const entity = { deviceId, fingerprint, email, license: "Cracked_By_DreamNya",
                     version, date, type: "DreamNya" };
    if (__getRealFp()) entity.fingerprint = __getRealFp();   // 运行时真指纹覆盖
    entity.date = ...当天...;
    return Buffer.from(JSON.stringify(entity));              // 伪"解密结果"
};
```

---

## 五、main.node 的真实身份（排除项）

| 检查项 | 结果 |
|---|---|
| PE 结构 | 9 节（.text 743KB / .rdata 131KB / .data 175KB…），无加壳（64KB 段熵 >7.5 占比 **0%**） |
| 导出表 | 仅 `napi_register_module_v1` + `node_api_module_get_api_version_v1`（标准 NAPI 入口） |
| 字符串表 | `isHunspell` / `isMisspelled` / `setDictionary` / `getCorrectionsForMisspelling` / 拼写字母表 `AAaBBbCCc…` |
| 中文串 | 0 |
| 授权相关串 | 0 |

**结论：main.node 是拼写检查助手（Hunspell 封装）+ 文件助手，与授权验证无关。** 验签的"原生实现"并不存在——`jsDecrypt` 是 Typora 自己用 JS 写的 RSA（PKCS#1 系列错误文案是其内嵌实现）。

---

## 六、"不用机器码"的路线评估（按侵入深度）

| # | 路线 | 状态 | 评估 |
|---|---|---|---|
| 1 | **Hook `publicDecrypt` 返回构造实体**（现方案） | ✅ 已实现并真机验证 | 最稳：不碰字节码、不碰原生层、机器码全自动 |
| 2 | Hook 验签结果的"消费方"（`[L] pass~` 判断处） | 理论可行 | 效果同 1，但离实现细节更近，版本更新更易碎。不推荐 |
| 3 | 构造免验签的 SLicense | 理论可行 | 需先反编译 checksum 算法，收益为零（1 已覆盖） |
| 4 | 修改 `atom.compiled.dist.jsc` | ❌ 不可行 | V8 字节码跨版本不可加载、无通用反编译工具；且授权校验走的都是 Node API，在更早层面即可劫持 |
| 5 | 二进制层改 Typora.exe | ❌ 不划算 | 验签逻辑在 JS 层不在原生层（所有锚点均可在 JS 层 hook 到） |
| 6 | 伪造官方服务器响应 | ❌ 死路 | 私钥不可得；且本地 `publicDecrypt` hook 已覆盖同样的效果 |

**结论**：1.14.10 上"无机器码方案"就是路线 1，且已是最优解。它唯一的软肋是"本地 RSA 验签"这个前提本身——只要 Typora 不把它改成纯在线强校验（每次启动问服务器且本地无可伪造状态），这套方案就不用动。

---

## 七、未来版本变动的风险与预案

| 变动 | 概率 | 冲击 | 预案 |
|---|---|---|---|
| Electron 大版本升级 | 高（不受 Typora 控制） | fuses/V8/Node API 面变化 | 方案只依赖 `crypto`/`fs`/`net` 稳定 API；每次升级跑全套测试即可发现 |
| 机器码字段变（`{v,i,l}` → 其它） | 中 | 自动生成的机器码字段不匹配 | 改 `patcher.AutoMachineCode` 的字段映射；症状与处置见 DOC/TROUBLESHOOTING.md |
| 验签改纯在线强校验 | 低（离线优先设计与之矛盾） | 路线 1 失效 | 兜底：绿色版冻结 1.14.10（更新已拦截）+ `fax\typora-his\` 旧版母本；或购买正版（$14.99 买断 1.x） |
| 启用 `EnableEmbeddedAsarIntegrityValidation` fuse | 低 | 解包目录路线受影响 | fuse 本身在 exe 里仍可翻转；真启用时需重新评估 |

---

## 附录 A：`launch.dist.js` 官方原文（1,383 字节）

```js
"use strict";const fs=require("fs"),path=require("path"),vm=require("vm"),v8=require("v8"),Module=require("module");v8.setFlagsFromString("--no-lazy"),v8.setFlagsFromString("--no-flush-bytecode");let dummyBytecode;function setFlagHashHeader(e){if(!dummyBytecode){let t=new vm.Script("",{produceCachedData:!0});dummyBytecode=t.createCachedData()}dummyBytecode.slice(12,16).copy(e,12)}function getSourceHashHeader(e){return e.slice(8,12)}function buffer2Number(e){let t=0;return t|=e[3]<<24,t|=e[2]<<16,t|=e[1]<<8,t|=e[0]}Module._extensions[".jsc"]=Module._extensions[".cjsc"]=function(e,t){let a=fs.readFileSync(t);if(!Buffer.isBuffer(a))throw Error("BytecodeBuffer must be a buffer object.");setFlagHashHeader(a);let r=buffer2Number(getSourceHashHeader(a)),o="";r>1&&(o='"'+"​".repeat(r-2)+'"');let c=new vm.Script(o,{filename:t,lineOffset:0,displayErrors:!0,cachedData:a});if(c.cachedDataRejected)throw Error("Invalid or incompatible cached data (cachedDataRejected)");let n=function(t){return e.require(t)};n.resolve=function(t,a){return Module._resolveFilename(t,e,!1,a)},process.mainModule&&(n.main=process.mainModule),n.extensions=Module._extensions,n.cache=Module._cache;let s=c.runInThisContext({filename:t,lineOffset:0,columnOffset:0,displayErrors:!0}),u=path.dirname(t),d=[e.exports,n,e,t,u,process,global];return s.apply(e.exports,d)},require("./atom.compiled.dist.jsc");
```

## 附录 B：授权链路关键字符串锚点（jsc 内偏移）

| 偏移 | 字符串 | 意义 |
|---|---|---|
| 87,138 | `[WindowsLicenseLocalStore] ` | 本地授权存取日志前缀 |
| 89,618 | `publicDecrypt` | **RSA 公钥解密 —— 主 Hook 点** |
| 93,078 | `base64` | SLicense 字段解码 |
| 107,673 | `trailRemains is ` | 试用剩余天数 |
| 108,382 | `[L] pass~@` | 校验通过标记 |
| 121,040 | `license.machineCode` | 机器码 IPC 入口 |
| — | `[L] validate server return empty/fail` | 服务器校验分支 |
| — | `[renewL]: unfill due to renew fail` | 续期失败 → 清空 SLicense |
| — | `^([A-Z0-9]{6}-){3}[A-Z0-9]{6}$` | license code 格式 |
| — | `L23456789ABCDEFGHJKMNPQRSTUVWXYZ` | license code 字母表（去 I/O/0/1） |
| — | `checksum failed` | license code 本地校验和 |
| — | `offlineActivation` | 离线激活入口 |
| — | `OUT_OF_LIMIT` / `INVALIDATE` / `WRONG_USER` | 服务器状态码 |
| — | `SOFTWARE\Microsoft\Cryptography` + `MachineGuid` | 指纹原料 |

## 附录 C：SLicense 格式解析

补丁写入值：`RHJlYW1OeWE=#0#1/1/2059`，按 `#` 分三段：

| 段 | 值 | 解读 |
|---|---|---|
| 1 | `RHJlYW1OeWE=` | base64，解码 = `DreamNya`（补丁作者署名；Typora 对此段只透传展示，本地不做校验） |
| 2 | `0` | 计数器/设备序号 |
| 3 | `1/1/2059` | 许可有效期（远期，等效永久） |

## 附录 D：分析过程存档

- 全量字符串表：`%TEMP%\jsc-strings.txt`（1,859 条）
- 侦察脚本：`%TEMP%\recon-license.js`、`%TEMP%\main-node-recon.js`
- 关键否证记录：本机 Node 24.21.0 加载 jsc → `cachedDataRejected: true`（跨 V8 不可用）
