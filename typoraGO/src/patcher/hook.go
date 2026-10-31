package patcher

// 注入模板：与 typora_crack.js（v1.0.6）里的 getInsertCode 逐段对应，
// 只把 JS 模板字面量里多一层的转义还原成最终写进 launch.dist.js 的代码。
// 改这里等于改补丁行为，改完必须跑 parity_test（与仓库 JS 版逐字节比对）。

import (
	"encoding/json"
	"strings"
)

// entityJSON 生成与 JS 版 JSON.stringify(ACT_ENTITY) 等价的对象文本。
func entityJSON(e Entity) string {
	type ordered struct {
		DeviceID    string `json:"deviceId"`
		Fingerprint string `json:"fingerprint"`
		Email       string `json:"email"`
		License     string `json:"license"`
		Version     string `json:"version"`
		Date        string `json:"date"`
		Type        string `json:"type"`
	}
	b, _ := json.Marshal(ordered{
		DeviceID:    e.DeviceID,
		Fingerprint: e.Fingerprint,
		Email:       e.Email,
		License:     "Cracked_By_DreamNya",
		Version:     e.Version,
		Date:        e.Date,
		Type:        "DreamNya",
	})
	// JS 的 JSON.stringify 不转义 < > &，Go 默认会转义，还原回来
	s := string(b)
	s = strings.ReplaceAll(s, `\u003c`, "<")
	s = strings.ReplaceAll(s, `\u003e`, ">")
	s = strings.ReplaceAll(s, `\u0026`, "&")
	return s
}

// BuildHookCode 拼出注入代码。ent 为激活实体，enableHookDebug 对应 JS 的 EnableHookDebug。
func BuildHookCode(ent Entity, enableHookDebug bool) string {
	entity := entityJSON(ent)
	debug := "false"
	if enableHookDebug {
		debug = "true"
	}
	code := hookTemplate
	code = strings.ReplaceAll(code, "__ENTITY__", entity)
	code = strings.ReplaceAll(code, "__DEBUG__", debug)
	return code
}

// InjectedMarker 是注入代码的起始标记（status 检测与重复注入防护都用它）。
const InjectedMarker = "Hook破解开始"

const hookTemplate = `

/** Hook破解开始 */
// 【v1.0.5 绿色版】运行时取本机真实 fingerprint（Typora 内部 machineCode 的 i 字段）。
// Typora 本地校验只比对 fingerprint，不比对 deviceId；激活实体里的 fingerprint 必须
// 等于本机运行时值（换机器即不同），否则解 SLicense 后比对失败 → onUnfillLicense 回试用。
let __realFp = null;
function __getRealFp() {
    if (__realFp) return __realFp;
    try {
        const h = electron.ipcMain._invokeHandlers;
        const fn = h && h.get && h.get("license.machineCode");
        if (fn) {
            const fakeEvent = { sender: { send: function(){}, isDestroyed: function(){ return true; } }, frameId: 0, processId: 0, reply: function(){} };
            const ret = fn(fakeEvent);
            if (ret && typeof ret.then === "function") {
                ret.then(function (mc) {
                    try { __realFp = JSON.parse(Buffer.from(mc, "base64").toString("utf8")).i; } catch (e) {}
                }).catch(function () {});
            }
        }
    } catch (e) {}
    return __realFp;
}

const electron = require("electron");
// 【v1.0.6 修复·关键】机器码是"异步"查出来的（handler 返回 Promise），而 license 校验是
// 同步取 fingerprint 的。原来只在 hook 里临时调一次，永远慢半拍：实体构建时 __realFp 还是
// null，于是只能用烘焙在文件里的旧指纹 —— 只有构建机（烘焙值正好等于本机值）能通过，
// 换任何机器都会解出 no info → onUnfillLicense → 回试用 15 天（issue #2 用户实测）。
// 对策：① 在 jsc 加载前就开轮询预热，handler 一注册就把真指纹查出来缓存，赶在首次校验前就位；
//       ② 查到后落一份到注册表，之后每次启动同步命中（指纹跨机器固定、跨版本稳定），彻底消除竞态。
const __FP_KEY = "HKCU\\Software\\Typora";
const __FP_VALUE = "MFp";
function __fpExec(cmd) {
    try {
        const { execSync } = require("child_process");
        return execSync(cmd, { windowsHide: true, encoding: "utf8" });
    } catch (e) { return ""; }
}
if (process.platform === "win32") {
    const cached = __fpExec('reg query "' + __FP_KEY + '" /v ' + __FP_VALUE).match(/MFp\s+REG_SZ\s+(\S+)/);
    if (cached && cached[1]) __realFp = cached[1];
}
(function __warmRealFp() {
    let tries = 0, called = false, done = false;
    const timer = setInterval(function () {
        tries++;
        if (done || tries > 1200) { clearInterval(timer); return; }
        try {
            const h = electron.ipcMain && electron.ipcMain._invokeHandlers;
            const fn = h && h.get && h.get("license.machineCode");
            if (!fn || called) return;
            called = true;
            const ret = fn({ sender: { send: function(){}, isDestroyed: function(){ return true; } }, frameId: 0, processId: 0, reply: function(){} });
            if (ret && typeof ret.then === "function") {
                ret.then(function (mc) {
                    try {
                        const fp = JSON.parse(Buffer.from(mc, "base64").toString("utf8")).i;
                        if (fp) {
                            __realFp = fp;
                            if (process.platform === "win32") __fpExec('reg add "' + __FP_KEY + '" /v ' + __FP_VALUE + ' /t REG_SZ /d "' + fp + '" /f');
                        }
                    } catch (e) {}
                    done = true;
                }).catch(function () { done = true; });
            } else { done = true; }
        } catch (e) {}
    }, 8);
    if (timer && timer.unref) timer.unref();
})();
// 【2nd 屏蔽】Typora 启动后约 530 秒触发一次 "2nd" 二次校验，其中约 20% 概率
// 无条件吊销授权（该分支不读注册表，补写 SLicense 无效），运行中弹「试用 0 天」。
// 拦截启动早期注册的、延迟在 8分40秒~9分05秒 之间的第一个定时器，使其永不触发。
const __bootAt = Date.now();
const __origSetTimeout = global.setTimeout;
const __fakeTimer = { ref: function(){}, unref: function(){}, hasRef: function(){ return false; }, refresh: function(){ return this; }, _destroyed: true };
let __2ndBlocked = false;
function __hookSetTimeout(fn, delay) {
    if (!__2ndBlocked && typeof delay === "number" && delay >= 520000 && delay <= 545000 && Date.now() - __bootAt < 120000) {
        __2ndBlocked = true;
        return __fakeTimer;
    }
    return __origSetTimeout.apply(this, arguments);
}
global.setTimeout = __hookSetTimeout;
try {
    const __timers = require("timers");
    if (__timers && __timers.setTimeout !== __hookSetTimeout) __timers.setTimeout = __hookSetTimeout;
} catch (e) {}
if (__DEBUG__) {
    Object.defineProperty(electron.app, "quit", {
        value: function () {},
        writable: true,
        configurable: true,
    });
}
electron.app.on("browser-window-created", (_event, win) => {
    win.webContents.once("dom-ready", () => {
        // 【静默化】窗口 DOM 就绪前再补一次 SLicense，确保渲染进程读到的是有效 license，不弹试用窗
        restoreSLicense();
        if (__DEBUG__) win.webContents.openDevTools({ mode: "detach" });
    });
});
const fsPathFrom = /resources[\\/]app[\\/]/i;
const fsPathTo = "resources\\app.bak\\";
const fsHook = {};
[
    "readFileSync","readFile","statSync","stat","Stats","StatsFs","open","openSync"
].forEach((property) => {
    fsHook[property] = fs[property];
    fs[property] = function (filePath, ...args) {
        if (typeof filePath == "string" && fsPathFrom.test(filePath)) {
            return fsHook[property].call(this, filePath.replace(fsPathFrom, fsPathTo), ...args);
        }
        return fsHook[property].call(this, filePath, ...args);
    };
});
const fsPromisesHook = {};
["readFile","open","stat"].forEach((property) => {
    fsPromisesHook[property] = fs.promises[property];
    fs.promises[property] = async function (filePath, ...args) {
        if (typeof filePath == "string" && fsPathFrom.test(filePath)) {
            return fsPromisesHook[property].call(this, filePath.replace(fsPathFrom, fsPathTo), ...args);
        }
        return fsPromisesHook[property].call(this, filePath, ...args);
    };
});
const crypto = require("crypto");
const originalPublicDecrypt = crypto.publicDecrypt;
crypto.publicDecrypt = function (key, buffer) {
    const entity = __ENTITY__;
    if (typeof __getRealFp === "function" && __getRealFp()) entity.fingerprint = __getRealFp();
    entity.date = (()=>{const d=new Date();return String(d.getMonth()+1).padStart(2,'0')+'/'+String(d.getDate()).padStart(2,'0')+'/'+d.getFullYear()})();
    return Buffer.from(JSON.stringify(entity));
};
// 额外 Hook: crypto.privateDecrypt (部分版本可能使用)
if (crypto.privateDecrypt) {
    const origPrivateDecrypt = crypto.privateDecrypt;
    crypto.privateDecrypt = function (key, buffer) {
        const entity = __ENTITY__;
        if (typeof __getRealFp === "function" && __getRealFp()) entity.fingerprint = __getRealFp();
        entity.date = (()=>{const d=new Date();return String(d.getMonth()+1).padStart(2,'0')+'/'+String(d.getDate()).padStart(2,'0')+'/'+d.getFullYear()})();
        return Buffer.from(JSON.stringify(entity));
    };
}
// 【v1.0.4 更新拦截】Typora 会静默自动更新（实测日志：启动约 17 秒后 checkForUpdates
// → 6 秒后自动下载 typora-update-x64-*.exe → 33 秒后自动装完重启），安装器把 Typora.exe
// 与 app.asar 换回官方新版后 fuses 复原、注入代码缺席，下次启动 RSA 验签失败即吊销授权，
// 表现为「第一天正常、第二天弹试用期剩余 X 天」。此处拦掉更新检查与安装包下载（4 个出口），
// 让更新器永远查不到新版本；需要升级 Typora 时先回滚补丁、装新版、再重跑本脚本即可。
// isUpdateUrl：识别 Typora 更新域名（typora.io / typoraio.cn）下的版本查询与下载请求
function isUpdateUrl(u) {
    if (!u) return false;
    if (u.indexOf("typora.io") === -1 && u.indexOf("typoraio.cn") === -1) return false;
    return u.indexOf("/releases/") !== -1
        || u.indexOf("downloads.typoraio") !== -1
        || u.indexOf("download.typora") !== -1
        || u.indexOf("typora-update") !== -1;
}
// 出口 1：Node 层 fetch 兜底
const __origGlobalFetch = globalThis.fetch;
if (typeof __origGlobalFetch === "function") {
    globalThis.fetch = function (input, init) {
        let u = "";
        if (typeof input === "string") u = input;
        else if (input && typeof input.url === "string") u = input.url;
        if (isUpdateUrl(u)) return Promise.resolve(new Response(null, { status: 204 }));
        return __origGlobalFetch.apply(this, arguments);
    };
}
// Hook: 拦截 electron.net.request (electron-fetch 默认使用此方法发请求!)
// electron-fetch 在 Electron 环境中默认 useElectronNet=true，调用 electron.net.request 而非 net.fetch
// 必须在 jsc 加载前同步设置，防止 electron-fetch 缓存原始引用
const origNetRequest = electron.net.request.bind(electron.net);
electron.net.request = function(options, callback) {
    let url = '';
    if (typeof options === 'string') url = options;
    else if (options && typeof options.url === 'string') url = options.url;
    else if (options && typeof options === 'object') {
        const proto = options.protocol || 'https:';
        const host = options.hostname || options.host || '';
        const p = options.path || '/';
        url = proto + '//' + host + p;
    }
    // 【v1.0.4】出口 2：更新请求返回 204 空响应（无 body，更新器解析失败即静默放弃）
    if (url && isUpdateUrl(url)) {
        const { EventEmitter } = require("events");
        const req204 = new EventEmitter();
        req204.write = function() { return req204; };
        req204.end = function() {
            const res204 = new EventEmitter();
            res204.statusCode = 204;
            res204.statusMessage = 'No Content';
            res204.headers = {};
            res204.setEncoding = function() {};
            res204.destroy = function() {};
            process.nextTick(function() {
                req204.emit('response', res204);
                process.nextTick(function() { res204.emit('end'); });
            });
            return req204;
        };
        req204.abort = function() {};
        req204.destroy = function() {};
        req204.setTimeout = function() { return req204; };
        req204.setHeader = function() {};
        req204.getHeader = function() { return undefined; };
        req204.removeHeader = function() {};
        req204.flushHeaders = function() {};
        return req204;
    }
    if (url && url.includes('api/client/')) {
        const dynEntity = __ENTITY__;
        if (typeof __getRealFp === "function" && __getRealFp()) dynEntity.fingerprint = __getRealFp();
        dynEntity.date = (()=>{const d=new Date();return String(d.getMonth()+1).padStart(2,'0')+'/'+String(d.getDate()).padStart(2,'0')+'/'+d.getFullYear()})();
        const dynMsg = Buffer.from(JSON.stringify(dynEntity)).toString('base64');
        const resBody = JSON.stringify({ success: true, code: 0, retry: true, msg: dynMsg });
        const { EventEmitter } = require("events");
        const mockReq = new EventEmitter();
        mockReq.write = function() { return mockReq; };
        mockReq.end = function() {
            const mockRes = new EventEmitter();
            mockRes.statusCode = 200;
            mockRes.statusMessage = 'OK';
            mockRes.headers = { 'content-type': 'application/json' };
            mockRes.setEncoding = function() {};
            mockRes.destroy = function() {};
            mockRes.pipe = function(dest) {
                process.nextTick(function() {
                    dest.write(Buffer.from(resBody));
                    dest.end();
                });
                return dest;
            };
            process.nextTick(function() {
                mockReq.emit('response', mockRes);
                process.nextTick(function() {
                    mockRes.emit('data', Buffer.from(resBody));
                    mockRes.emit('end');
                });
            });
            return mockReq;
        };
        mockReq.abort = function() {};
        mockReq.destroy = function() {};
        mockReq.setTimeout = function() { return mockReq; };
        mockReq.setHeader = function() { return mockReq; };
        mockReq.getHeader = function() { return undefined; };
        mockReq.removeHeader = function() { return mockReq; };
        mockReq.flushHeaders = function() {};
        return mockReq;
    }
    return origNetRequest(options, callback);
};
// Hook 4: 定期恢复 SLicense（防止 Typora 运行时二次验证清空 license）
// Typora 有概率性 2nd 二次验证机制，验证 SLicense 签名失败会 unfill 清空注册表
// 清空后会弹出"试用期剩余 0 天"，故需高频 + 提前恢复，把空窗压到最小，实现静默
const SLICENSE_VALUE = "RHJlYW1OeWE=#0#1/1/2059";
function restoreSLicense() {
    try {
        const { execSync } = require("child_process");
        try {
            const val = execSync('reg query "HKCU\\Software\\Typora" /v SLicense', { windowsHide: true, encoding: 'utf8' });
            if (val.indexOf(SLICENSE_VALUE) === -1) {
                execSync('reg add "HKCU\\Software\\Typora" /v SLicense /t REG_SZ /d "' + SLICENSE_VALUE + '" /f', { windowsHide: true });
            }
        } catch(e) {
            execSync('reg add "HKCU\\Software\\Typora" /v SLicense /t REG_SZ /d "' + SLICENSE_VALUE + '" /f', { windowsHide: true });
        }
    } catch(e) {}
}
// 【静默化】app ready 之前就立即恢复一次，堵住"启动瞬间→首次恢复"的弹窗空窗
restoreSLicense();
// 【IDate 兜底】吊销发生后 Typora 按 IDate 计算试用剩余天数；IDate 是激活当天写死的，
// 过了 15 天窗口后一旦漏网触发吊销就会显示「试用 0 天」。把 IDate 保持为当天，
// 使剩余天数恒为 15 天，作为 2nd 屏蔽万一失效时的兜底（每天只需实际写一次注册表）。
let __idateToday = null;
function refreshIDate() {
    try {
        const d = new Date();
        const val = String(d.getMonth() + 1).padStart(2, "0") + "/" + String(d.getDate()).padStart(2, "0") + "/" + d.getFullYear();
        if (val !== __idateToday) {
            __idateToday = val;
            const { execSync } = require("child_process");
            execSync('reg add "HKCU\\Software\\Typora" /v IDate /t REG_SZ /d "' + val + '" /f', { windowsHide: true });
        }
    } catch (e) {}
}
refreshIDate();
setInterval(refreshIDate, 30 * 60 * 1000);
// SLicense 恢复降频：二次验证右路不读注册表，恢复仅保证"下次启动"能读到有效值，5 秒一次足够
setInterval(restoreSLicense, 5000);
electron.app.whenReady().then(() => {
    // Hook 1: 拦截渲染进程发起的 https 请求
    electron.protocol.handle("https", async (request) => {
        // 【v1.0.4】出口 3：更新检查/下载直接回 204，让更新器静默放弃
        if (isUpdateUrl(request.url)) {
            return new Response(null, { status: 204 });
        }
        if (request.url.includes('api/client/')) {
            const dynEntity = __ENTITY__;
            if (typeof __getRealFp === "function" && __getRealFp()) dynEntity.fingerprint = __getRealFp();
            dynEntity.date = (()=>{const d=new Date();return String(d.getMonth()+1).padStart(2,'0')+'/'+String(d.getDate()).padStart(2,'0')+'/'+d.getFullYear()})();
            const dynMsg = Buffer.from(JSON.stringify(dynEntity)).toString('base64');
            return new Response(JSON.stringify({
                success: true, code: 0, retry: true, msg: dynMsg
            }), { status: 200 });
        }
        return electron.net.fetch(request, { bypassCustomProtocolHandlers: true });
    });
    // Hook 2: 拦截主进程 net.fetch 调用 (关键修复!)
    const origNetFetch = electron.net.fetch.bind(electron.net);
    electron.net.fetch = function(input, init) {
        let url = '';
        if (typeof input === 'string') url = input;
        else if (input && typeof input.url === 'string') url = input.url;
        // 【v1.0.4】出口 4：更新检查/下载直接回 204
        if (url && isUpdateUrl(url)) {
            return Promise.resolve(new Response(null, { status: 204 }));
        }
        if (url && url.includes('api/client/')) {
            const dynEntity = __ENTITY__;
            if (typeof __getRealFp === "function" && __getRealFp()) dynEntity.fingerprint = __getRealFp();
            dynEntity.date = (()=>{const d=new Date();return String(d.getMonth()+1).padStart(2,'0')+'/'+String(d.getDate()).padStart(2,'0')+'/'+d.getFullYear()})();
            const dynMsg = Buffer.from(JSON.stringify(dynEntity)).toString('base64');
            return Promise.resolve(new Response(JSON.stringify({
                success: true, code: 0, retry: true, msg: dynMsg
            }), { status: 200, headers: { 'content-type': 'application/json' } }));
        }
        return origNetFetch(input, init);
    };
    // Hook 3: 清理试用期相关 localStorage 数据
    try {
        const { session: { defaultSession } } = electron;
        if (defaultSession) {
            defaultSession.clearStorageData({ storages: ['localstorage', 'websql', 'indexdb'] }).catch(()=>{});
        }
    } catch(e) {}
    // ready 后再补一次恢复（双保险）
    restoreSLicense();
});
/** Hook破解结束 */
`
