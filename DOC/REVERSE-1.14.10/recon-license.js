// 逆向勘察 1.14.10 的授权校验链路（只读分析，不改任何文件）
// 产物：还原 launch.dist.js + jsc 里的字符串证据，画出调用关系
const fs = require("fs");
const path = require("path");

const APP = process.env.APP_DIR || "D:/Apps/Typora/resources/app.bak";

// ---------- 1. 解析 asar 头（原始归档，未打补丁的官方原件） ----------
const asarPath = "D:/Apps/Typora/resources/app.asar.bak";
const fd = fs.openSync(asarPath, "r");
const pre = Buffer.alloc(8);
fs.readSync(fd, pre, 0, 8, 0);
const headerSize = pre.readUInt32LE(4);
const hdr = Buffer.alloc(headerSize);
fs.readSync(fd, hdr, 0, headerSize, 8);
const payload = hdr.readUInt32LE(0);
const inner = headerSize - payload;
const jsonLen = hdr.readUInt32LE(inner);
const header = JSON.parse(hdr.slice(inner + 4, inner + 4 + jsonLen).toString("utf8"));
const dataStart = 8 + headerSize;

function readFileFromAsar(name) {
  const e = header.files[name];
  if (!e) return null;
  const buf = Buffer.alloc(e.size);
  fs.readSync(fd, buf, 0, e.size, dataStart + parseInt(e.offset, 10));
  return buf;
}

console.log("=== asar 包内文件 ===");
for (const [k, v] of Object.entries(header.files)) {
  console.log(`  ${k}  size=${v.size}  offset=${v.offset}`);
}

// ---------- 2. launch.dist.js（官方加载器，明文 JS） ----------
const launch = readFileFromAsar("launch.dist.js").toString("utf8");
console.log("\n=== launch.dist.js（" + launch.length + " 字节）完整内容 ===");
console.log(launch);

// ---------- 3. jsc 字节码里的字符串证据（V8 cached data 不反编译，但字符串表是明文） ----------
const jsc = readFileFromAsar("atom.compiled.dist.jsc");
console.log("\n=== jsc 基本信息 ===");
console.log("  大小:", jsc.length, "字节");
console.log("  magic:", jsc.slice(0, 8).toString("hex"));

// V8 cached data 的字符串以 UTF-16/latin1 混合存在；把可见串都拖出来
function extractStrings(buf, minLen) {
  const out = [];
  // latin1 可见串
  let cur = [];
  for (let i = 0; i < buf.length; i++) {
    const c = buf[i];
    if (c >= 32 && c < 127) { cur.push(String.fromCharCode(c)); }
    else { if (cur.length >= minLen) out.push(cur.join("")); cur = []; }
  }
  if (cur.length >= minLen) out.push(cur.join(""));
  return out;
}
const strs = extractStrings(jsc, 8);
console.log("  可见字符串（>=8 字符）总数:", strs.length);

// ---------- 4. 授权链路关键锚点检索 ----------
const anchors = [
  "license", "machineCode", "publicDecrypt", "privateDecrypt", "decrypt",
  "SLicense", "IDate", "MFp", "api/client", "renew", "trial", "trialRemains",
  "[L] pass", "[L] installDate", "2nd", "activate", "onUnfillLicense",
  "no info", "BEGIN PUBLIC", "BEGIN RSA", "setPublicKey", "setPrivateKey",
  "signature", "verify", "createVerify", "createDecipheriv", "aes",
  "fingerprint", "deviceId", "license_", "checkLicense", "fillLicense",
  "unfillLicense", "WindowsLicenseLocalStore", "getMachineCode",
];
console.log("\n=== 授权链路锚点命中（jsc 字符串表）===");
const found = new Map();
for (const s of strs) {
  for (const a of anchors) {
    if (s.includes(a)) {
      if (!found.has(a)) found.set(a, []);
      found.get(a).push(s.length <= 160 ? s : s.slice(0, 160) + "...");
    }
  }
}
for (const [a, hits] of found) {
  console.log(`\n  [${a}]  ${hits.length} 处`);
  for (const h of hits.slice(0, 6)) console.log("    ·", JSON.stringify(h));
}

// RSA 公钥本体（PEM 头）单独找 —— 这决定"本地验签"用的是什么密钥
console.log("\n=== PEM/密钥材料检索 ===");
for (const s of strs) {
  if (/BEGIN|^MII|publicKey|privateKey|setKey/i.test(s) && s.length > 40) {
    console.log("  ·", JSON.stringify(s.slice(0, 200)));
  }
}

// ---------- 5. 把字符串证据按出现顺序写文件，供细读 ----------
const dump = path.join(process.env.TEMP || ".", "jsc-strings.txt");
fs.writeFileSync(dump, strs.join("\n"), "utf8");
console.log("\n全部字符串已写入:", dump, "(", strs.length, "条 )");
fs.closeSync(fd);
