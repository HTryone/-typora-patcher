// ================================
// 自动检测并安装依赖
// ================================
const fs = require("fs");
const path = require("path");
const { spawnSync } = require('child_process');
const os = require('os');

const REQUIRED_DEPS = ["asar", "chalk@4", "readline-sync", "iconv-lite", "@electron/fuses"];
const NODE_MODULES_DIR = path.join(__dirname, 'node_modules');

if (!fs.existsSync(NODE_MODULES_DIR)) {
    console.log("📦 首次运行，正在安装依赖...");
    // npm init
    const pkgFile = path.join(__dirname, 'package.json');
    if (!fs.existsSync(pkgFile)) {
        const init = spawnSync('npm', ['init', '-y'], { cwd: __dirname, stdio: 'inherit', shell: true });
        if (init.status !== 0) {
            console.error("❌ npm init 失败，请手动运行：npm init -y");
            process.exit(1);
        }
    }
    // npm install
    const install = spawnSync('npm', ['install', ...REQUIRED_DEPS], { cwd: __dirname, stdio: 'inherit', shell: true });
    if (install.status !== 0) {
        console.error("❌ 依赖安装失败，请手动运行：npm install " + REQUIRED_DEPS.join(" "));
        process.exit(1);
    }
    console.log("✅ 依赖安装完成！重新启动脚本...\n");
    // 重新启动自身，让 require 能加载新安装的模块
    const relaunch = spawnSync(process.execPath, [process.argv[1], ...process.argv.slice(2)], {
        cwd: __dirname,
        stdio: 'inherit',
        shell: false
    });
    process.exit(relaunch.status || 0);
}

const asar = require("asar");
const chalk = require("chalk");
const readlineSync = require("readline-sync");
const iconv = require('iconv-lite');
const { flipFuses, FuseV1Options, FuseVersion } = require("@electron/fuses");

// ================================
// 【修正】Windows 控制台乱码
// ================================
if (process.platform === "win32") {
    process.stdout.setEncoding('utf8');
    process.stdin.setEncoding('utf8');
    process.stderr.setEncoding('utf8');
    try {
        spawnSync('cmd.exe', ['/c', 'chcp', '65001'], {
            stdio: 'ignore',
            shell: false,
            windowsHide: true
        });
    } catch (e) {}
    // 【关键修复】正确配置 readline-sync 的 I/O 流
    readlineSync.setDefaultOptions({
        input: process.stdin,
        output: process.stdout
    });
}

// ================================
// 【新增】使用内置 child_process 操作注册表（替代 winreg）
// ================================

/**
 * 向注册表写入一个字符串值 (REG_SZ)
 * @param {string} key - 注册表键路径，例如 "HKCU\\Software\\Typora"
 * @param {string} name - 值名称
 * @param {string} value - 值内容
 */
function writeRegValue(key, name, value) {
    if (process.platform !== 'win32') return;
    try {
        const result = spawnSync('reg', [
            'add', key,
            '/v', name,
            '/t', 'REG_SZ',
            '/d', value,
            '/f' // 强制覆盖，不提示
        ], {
            stdio: 'ignore',
            windowsHide: true,
            shell: false
        });

        if (result.status !== 0) {
            throw new Error(`reg add command failed with code ${result.status}`);
        }
    } catch (error) {
        console.log(chalk.red(`❌ 写入注册表失败: ${error.message}`));
        throw error;
    }
}

/**
 * 从注册表删除一个值
 * @param {string} key - 注册表键路径
 * @param {string} name - 要删除的值名称
 */
function deleteRegValue(key, name) {
    if (process.platform !== 'win32') return;
    try {
        const result = spawnSync('reg', [
            'delete', key,
            '/v', name,
            '/f' // 强制删除，不提示
        ], {
            stdio: 'ignore',
            windowsHide: true,
            shell: false
        });
        // 即使值不存在，reg delete 也会返回非0，但我们不认为这是错误
    } catch (error) {
        // 清理失败通常不影响回滚主流程，可以忽略
    }
}

// ================================
// 缓存路径配置 (扩展以支持激活信息)
// ================================
const CONFIG_FILE = path.join(os.homedir(), '.typora_crack_config.json');

// 【修改】readCache 现在读取整个缓存对象
function readCache() {
    try {
        if (fs.existsSync(CONFIG_FILE)) {
            const cacheData = JSON.parse(fs.readFileSync(CONFIG_FILE, 'utf8'));
            // 验证路径有效性
            if (cacheData.path && fs.existsSync(path.join(cacheData.path, 'Typora.exe'))) {
                return {
                    path: path.resolve(cacheData.path),
                    machineCode: cacheData.machineCode || null,
                    email: cacheData.email || null
                };
            }
        }
    } catch (e) {}
    return { path: null, machineCode: null, email: null };
}

// 【修改】saveCache 现在接受完整的缓存对象
function saveCache(cacheObj) {
    try {
        fs.writeFileSync(CONFIG_FILE, JSON.stringify(cacheObj, null, 2), 'utf8');
    } catch (e) {}
}

// ================================
// 从注册表读取路径（自动去掉 Typora.exe 后缀）
// ================================
function readRegPath() {
    if (process.platform !== 'win32') return null;
    try {
        // 【关键修改】不再使用 encoding: 'utf8'
        const r = spawnSync('reg', [
            'query', 'HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\Typora.exe', '/ve'
        ], {
            // encoding: 'utf8', // <-- 注释掉或删除这一行
            windowsHide: true
        });
        
        if (r.status === 0) {
            // 【关键修改】使用 iconv-lite 将 GBK 编码的 Buffer 转换为 UTF-8 字符串
            let stdoutStr = iconv.decode(r.stdout, 'cp936'); // 'cp936' 是 GBK 的别名
            
            // --- 调试日志 ---
            //console.log("【DEBUG】解码后的 stdout:", JSON.stringify(stdoutStr));
            // --- 调试日志结束 ---
            for (const line of stdoutStr.split('\n')) {
                const m = line.match(/REG_SZ\s+(.+)/);
                if (m) {
                    let p = m[1].trim();
                    if (p.endsWith('Typora.exe')) p = path.dirname(p);
                    p = path.resolve(p);
                    if (fs.existsSync(path.join(p, 'Typora.exe'))) return p;
                }
            }
        } else {
            console.log(chalk.yellow(`⚠️  reg query 命令执行失败，退出码: ${r.status}`));
            console.log(chalk.yellow(`stderr: ${r.stderr}`));
        }
    } catch (e) {
        console.log(chalk.yellow(`⚠️  读取注册表时发生异常: ${e.message}`));
    }
    return null;
}

// ================================
// 选择 Typora 安装目录（严格按你要求的顺序）
// ================================
function chooseTyporaDir() {
    process.stdout.write('\x1Bc');
    console.log(chalk.cyanBright("================================="));
    console.log(chalk.magentaBright("        Typora 激活脚本          "));
    console.log(chalk.cyanBright("================================="));
    // 1. 先读缓存
    const cache = readCache();
    if (cache.path) {
        console.log(chalk.green(`✅ 检测到缓存路径: ${cache.path}`));
        console.log(chalk.cyan("是否使用此路径？[Y/n]："));
        const useCache = readlineSync.question("").trim().toLowerCase();
        if (useCache !== 'n' && useCache !== 'no') {
            console.log(chalk.greenBright(`✅ 已选择：${cache.path}`));
            return cache.path;
        }
        console.log(chalk.yellow("已跳过缓存路径，正在尝试从注册表读取..."));
    }
    // 2. 再读注册表
    const regPath = readRegPath();
    if (regPath) {
        console.log(chalk.green(`✅ 从注册表检测到路径: ${regPath}`));
        console.log(chalk.cyan("是否使用此路径？[Y/n]："));
        const useReg = readlineSync.question("").trim().toLowerCase();
        if (useReg !== 'n' && useReg !== 'no') {
            console.log(chalk.greenBright(`✅ 已选择：${regPath}`));
            // 保存新路径，但保留原有的激活信息（如果有的话）
            saveCache({ ...cache, path: regPath });
            return regPath;
        }
        console.log(chalk.yellow("已跳过注册表路径，进入文件夹选择窗口..."));
    } else {
        console.log(chalk.yellow("未在注册表中找到 Typora，准备打开文件夹选择窗口..."));
    }
    // 3. 最后打开文件夹选择窗口
    try {
        console.log(chalk.blueBright("正在打开文件夹选择窗口..."));
        const psCommand = `
            [Console]::OutputEncoding = [System.Text.Encoding]::UTF8;
            Add-Type -AssemblyName System.Windows.Forms;
            $fbd=New-Object System.Windows.Forms.FolderBrowserDialog;
            $fbd.Description='请选择 Typora 安装目录';
            $fbd.ShowNewFolderButton=$false;
            if($fbd.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){
                Write-Output $fbd.SelectedPath
            }
        `.replace(/\s+/g, ' ').trim();
        const output = spawnSync('powershell.exe', [
            '-NoProfile',
            '-ExecutionPolicy', 'Bypass',
            '-Command', psCommand
        ], {
            encoding: 'utf8',
            windowsHide: false,
            shell: false,
            stdio: ['ignore', 'pipe', 'ignore'],
            env: { ...process.env, PYTHONIOENCODING: 'utf-8', LC_ALL: 'en_US.UTF-8' }
        }).stdout.trim();
        if (!output) {
            throw new Error("未选择路径");
        }
        const cleanPath = path.resolve(output);
        console.log(chalk.greenBright(`✅ 已选择：${cleanPath}`));
        // 保存新路径，但保留原有的激活信息（如果有的话）
        saveCache({ ...cache, path: cleanPath });
        return cleanPath;
    } catch (err) {
        console.log(chalk.redBright("\n❌ 图形窗口不可用，切换手动输入"));
        return manualInputPath(cache);
    }
}
function manualInputPath(cache) {
    while (true) {
        console.log(chalk.cyan("请输入 Typora 安装目录路径（例如 C:\\\\Program Files\\\\Typora）: "));
        const input = readlineSync.question("");
        if (input.trim()) {
            const res = path.resolve(input.trim());
            if (fs.existsSync(path.join(res, 'Typora.exe'))) {
                // 保存新路径，但保留原有的激活信息（如果有的话）
                saveCache({ ...cache, path: res });
                return res;
            } else {
                console.log(chalk.red("❌ 该路径下无效，请重新输入！"));
            }
        }
        console.log(chalk.red("路径不能为空，请重新输入！"));
    }
}

// ================================
// 【新增】获取机器码和邮箱，并加入缓存逻辑
// ================================
function getMachineCodeAndEmail() {
    const cache = readCache();
    let machineCode = null;
    let email = null;

    // 尝试从缓存中读取
    if (cache.machineCode && cache.email) {
        console.log(chalk.green(`\n✅ 检测到缓存的激活信息:`));
        console.log(chalk.green(`   机器码: ${cache.machineCode}`));
        console.log(chalk.green(`   邮箱: ${cache.email}`));
        console.log(chalk.cyan("是否使用此信息？[Y/n]："));
        const useCache = readlineSync.question("").trim().toLowerCase();
        if (useCache !== 'n' && useCache !== 'no') {
            console.log(chalk.greenBright("✅ 已使用缓存的激活信息！"));
            return { machineCode: cache.machineCode, email: cache.email };
        }
        console.log(chalk.yellow("已跳过缓存的激活信息，进入手动输入..."));
    }

    // 手动输入，并增加非空校验
    while (true) {
        console.log(chalk.cyanBright("\n请输入机器码: "));
        machineCode = readlineSync.question().trim(); // 使用 trim() 去除首尾空格
        
        console.log(chalk.cyanBright("请输入邮箱: "));
        email = readlineSync.question().trim();

        // 【新增】关键的非空判断
        if (!machineCode) {
            console.log(chalk.red("❌ 机器码不能为空，请重新输入！"));
            continue; // 跳过本次循环，重新开始输入
        }
        if (!email) {
            console.log(chalk.red("❌ 邮箱不能为空，请重新输入！"));
            continue; // 跳过本次循环，重新开始输入
        }

        // 如果都通过了验证，跳出循环
        break;
    }

    // 保存新的激活信息到缓存
    saveCache({ ...cache, machineCode, email });

    return { machineCode, email };
}

// ================================
// 回滚还原函数
// ================================
function rollbackTypora(Typora_Installation_Path) {
    console.log(chalk.yellowBright("\n🔄 开始执行回滚还原..."));
    const resourcesPath = path.join(Typora_Installation_Path, "resources");
    const asarPath = path.join(resourcesPath, "app.asar");
    const asarBakPath = path.join(resourcesPath, "app.asar.bak");
    const appDir = path.join(resourcesPath, "app");
    const appBakDir = path.join(resourcesPath, "app.bak");
    const TyporaEXE = path.join(Typora_Installation_Path, "Typora.exe");
    const exeBakPath = `${TyporaEXE}.bak`;
    try {
        spawnSync('taskkill', ['/F', '/IM', 'Typora.exe'], {
            stdio: 'ignore',
            windowsHide: true,
            shell: false
        });
        if (fs.existsSync(asarBakPath)) {
            if (fs.existsSync(asarPath)) fs.rmSync(asarPath, { force: true });
            fs.renameSync(asarBakPath, asarPath);
            console.log(chalk.green("✅ 已还原 app.asar"));
        }
        if (fs.existsSync(exeBakPath)) {
            if (fs.existsSync(TyporaEXE)) fs.rmSync(TyporaEXE, { force: true });
            fs.renameSync(exeBakPath, TyporaEXE);
            console.log(chalk.green("✅ 已还原 Typora.exe"));
        }
        if (fs.existsSync(appDir)) {
            fs.rmSync(appDir, { recursive: true, force: true });
            console.log(chalk.green("✅ 已删除解包 app 目录"));
        }
        if (fs.existsSync(appBakDir)) {
            fs.rmSync(appBakDir, { recursive: true, force: true });
        }
        try {
            // 【已修改】使用新的 deleteRegValue 函数
            deleteRegValue("HKCU\\Software\\Typora", "SLicense");
            deleteRegValue("HKCU\\Software\\Typora", "IDate");
            console.log(chalk.green("✅ 已清理授权信息"));
        } catch (e) {}
        console.log(chalk.greenBright("\n🎉 回滚完成！已恢复为官方原版！"));
        process.exit(0);
    } catch (err) {
        console.log(chalk.red("❌ 回滚失败："), err.message);
        process.exit(1);
    }
}

// ================================
// 路径验证 + 已激活检测 + Y/N回滚
// ================================
let Typora_Installation_Path = "";
let isAlreadyCracked = false;
while (true) {
    Typora_Installation_Path = chooseTyporaDir();
    const exePath = path.join(Typora_Installation_Path, "Typora.exe");
    const resourcesPath = path.join(Typora_Installation_Path, "resources");
    const asarPath = path.join(resourcesPath, "app.asar");
    const asarBakPath = path.join(resourcesPath, "app.asar.bak");
    if (!fs.existsSync(exePath)) {
        console.log(chalk.red("❌ 错误：该目录下没有找到 Typora.exe！"));
        console.log(chalk.yellow("请确保选择的是 Typora 的根安装目录。\n"));
        continue;
    }

    // 【修改后的判断逻辑】
    // 如果 app.asar 不存在，说明很可能已被激活（或文件损坏）
    if (!fs.existsSync(asarPath)) {
        // 额外检查一下是否存在 app 目录，以区分是“已激活”还是“文件损坏”
        const appDir = path.join(resourcesPath, "app");
        if (fs.existsSync(appDir)) {
            // app.asar 不存在，但 app 目录存在 -> 已激活
            console.log(chalk.yellowBright("\n⚠️  检测到 Typora 已经被激活过！"));
            console.log(chalk.cyan("是否执行回滚还原？[Y/N]："));
            const choice = readlineSync.question("").trim().toUpperCase();
            if (choice === 'Y') {
                rollbackTypora(Typora_Installation_Path);
            } else {
                console.log(chalk.green("已取消回滚，退出脚本。"));
                process.exit(0);
            }
        } else {
            // app.asar 和 app 目录都不存在 -> 文件可能损坏
            console.log(chalk.red("❌ 错误：未找到 resources/app.asar 文件，且无解包目录！"));
            console.log(chalk.yellow("请确认这是完整的 Typora 安装目录。\n"));
            continue;
        }
    }

    // 如果能走到这里，说明 app.asar 存在，可以继续激活
    console.log(chalk.green("✅ 路径验证成功！准备开始激活..."));
    break;
}

// ================================
// 激活逻辑
// ================================
function getInsertCode(EnableHookDebug, atobMachineCode, email, nowDateStr) {
    const ACT_ENTITY = {
        deviceId: `${atobMachineCode.l}`,
        fingerprint: `${atobMachineCode.i}`,
        email: `${email}`,
        license: "Cracked_By_DreamNya",
        version: `${atobMachineCode.v}`,
        date: `${nowDateStr}`,
        type: "DreamNya"
    };
    const StandardLicenseMsg = Buffer.from(JSON.stringify(ACT_ENTITY)).toString('base64');
    return `
/** Hook破解开始 */
const electron = require("electron");
if (${EnableHookDebug}) {
    Object.defineProperty(electron.app, "quit", {
        value: function () {},
        writable: true,
        configurable: true,
    });
}
electron.app.on("browser-window-created", (_event, win) => {
    win.webContents.once("dom-ready", () => {
        if (${EnableHookDebug}) win.webContents.openDevTools({ mode: "detach" });
    });
});
const fsPathFrom = /resources[\\\\/]app[\\\\/]/i;
const fsPathTo = "resources\\\\app.bak\\\\";
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
    return Buffer.from(JSON.stringify(${JSON.stringify(ACT_ENTITY)}));
};
electron.app.whenReady().then(() => {
    electron.protocol.handle("https", async (request) => {
        if (request.url.includes('api/client/activate') || request.url.includes('api/client/renew')) {
            return new Response(JSON.stringify({
                success: true, code: 0, retry: true, msg: "${StandardLicenseMsg}"
            }), { status: 200 });
        }
        return electron.net.fetch(request, { bypassCustomProtocolHandlers: true });
    });
});
/** Hook破解结束 */
`;
}
function closeTyporaProcesses() {
    try {
        spawnSync('taskkill', ['/F', '/IM', 'Typora.exe'], {
            stdio: 'ignore',
            windowsHide: true,
            shell: false
        });
        console.log(chalk.green("已关闭所有 Typora.exe 进程"));
    } catch (e) {
        console.log(chalk.red("Typora.exe 未运行，请手动关闭后按回车"));
        readlineSync.question('');
    }
}
// 【已移除】setRegValue 函数
function getNowDateStr() {
    const now = new Date();
    const dd = String(now.getDate()).padStart(2, "0");
    const mm = String(now.getMonth() + 1).padStart(2, "0");
    const yyyy = now.getFullYear();
    return `${mm}/${dd}/${yyyy}`;
}

// 【核心修改】替换原来的直接输入，调用带缓存的新函数
const { machineCode, email } = getMachineCodeAndEmail();

function atob(str) {
    return Buffer.from(str, "base64").toString("utf8");
}
const atobMachineCode = JSON.parse(atob(machineCode));
console.log(chalk.blueBright("deviceId: " + atobMachineCode.l));
console.log(chalk.blueBright("fingerprint: " + atobMachineCode.i));
console.log(chalk.blueBright("version: " + atobMachineCode.v));
const nowDateStr = getNowDateStr();
const EnableHookDebug = false;
closeTyporaProcesses();
const resourcesPath = path.join(Typora_Installation_Path, "resources");
const asarPath = path.join(resourcesPath, "app.asar");
const appDir = path.join(resourcesPath, "app");
const appBakDir = path.join(resourcesPath, "app.bak");
const asarBakPath = path.join(resourcesPath, "app.asar.bak");
const TyporaEXE = path.join(Typora_Installation_Path, "Typora.exe");
const LaunchDistJS = path.join(appDir, "launch.dist.js");
function generateRegCode() {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    let code = '+';
    for (let i = 0; i < 8; i++) code += chars.charAt(Math.floor(Math.random() * chars.length));
    return code + '#';
}

// 主逻辑
(async () => {
    try {
        process.stdout.write('\x1Bc');
        console.log(chalk.yellowBright("\n✨ 正在启动 Typora 激活脚本..."));
        console.log(chalk.greenBright("\n==== 开始处理... ===="));
        console.log(chalk.blueBright("\n一、环境配置..."));
        if (fs.existsSync(appDir)) fs.rmSync(appDir, { recursive: true, force: true });
        console.log(chalk.blue("解包 app.asar"));
        await asar.extractAll(asarPath, appDir);
        console.log(chalk.blue("备份 app 目录"));
        function copyDir(src, dest) {
            if (!fs.existsSync(dest)) fs.mkdirSync(dest, { recursive: true });
            for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
                const s = path.join(src, entry.name);
                const d = path.join(dest, entry.name);
                entry.isDirectory() ? copyDir(s, d) : fs.copyFileSync(s, d);
            }
        }
        copyDir(appDir, appBakDir);
        console.log(chalk.blue("备份原始 app.asar"));
        fs.renameSync(asarPath, asarBakPath);
        console.log(chalk.blue("配置 Electron Fuses..."));
        fs.copyFileSync(TyporaEXE, `${TyporaEXE}.bak`);
        await flipFuses(TyporaEXE, {
            version: FuseVersion.V1,
            [FuseV1Options.OnlyLoadAppFromAsar]: false,
        });
        console.log(chalk.greenBright("✅ 环境配置完成！"));
        console.log(chalk.blueBright("\n二、注入授权模块..."));
        let content = fs.readFileSync(LaunchDistJS, "utf-8");
        const match = content.match(/require\s*\([^)]*\)\s*;/);
        const code = getInsertCode(EnableHookDebug, atobMachineCode, email, nowDateStr);
        content = match
            ? content.slice(0, match.index + match[0].length) + code + content.slice(match.index + match[0].length)
            : code + content;
        fs.writeFileSync(LaunchDistJS, content, "utf-8");
        console.log(chalk.greenBright("✅ 注入完成！"));
        console.log(chalk.blueBright("\n三、写入授权..."));
        // 【已修改】使用新的 writeRegValue 函数
        writeRegValue("HKCU\\Software\\Typora", "SLicense", "RHJlYW1OeWE=#0#1/1/2059");
        writeRegValue("HKCU\\Software\\Typora", "IDate", nowDateStr);
        console.log(chalk.greenBright("✅ 授权写入成功！"));
        console.log(chalk.greenBright("\n🎉 全部完成！使用愉快！"));
        console.log(chalk.magentaBright(`🔑 激活码：${generateRegCode()}`));
    } catch (err) {
        console.error(chalk.red("\n💥 错误："), err);
        process.exit(1);
    }
})();