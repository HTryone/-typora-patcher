// main.node 导出面分析：从字符串表找 NAPI 属性注册用的方法名
const fs = require("fs");
const b = fs.readFileSync("D:/Apps/Typora/resources/app.asar.unpacked/main.node");
const strs = [];
let cur = [];
for (let i = 0; i < b.length; i++) {
  const c = b[i];
  if (c >= 32 && c < 127) cur.push(String.fromCharCode(c));
  else { if (cur.length >= 5) strs.push(cur.join("")); cur = []; }
}
const uniq = [...new Set(strs.filter(s => /^[A-Za-z_][A-Za-z0-9_]{4,39}$/.test(s)))];
console.log("独立可读词:", uniq.length);

// NAPI 属性注册时方法名会以 ASCII 出现；筛出像 API 名的
const apiLike = uniq.filter(s => /^(js|[a-z])[A-Z][a-zA-Z]+$/.test(s) || /^(decrypt|encrypt|verify|sign|license|machine|finger|reg)/i.test(s));
console.log("\n--- 疑似 API 名 ---");
apiLike.forEach(s => console.log("  " + s));

// RSA 相关常量（DER/PEM 特征：asn1 头 30 82）
let keyHits = 0;
for (let i = 0; i < b.length - 4; i++) {
  if (b[i] === 0x30 && b[i+1] === 0x82 && (b[i+2] & 0x80)) {
    const len = (b[i+2] << 8) | b[i+3];
    if (len > 100 && len < 4096) {
      // 看后续是不是模数样的高熵数据
      keyHits++;
      if (keyHits <= 8) console.log(`ASN.1 序列 @${i} len=${len}`);
      i += len;
    }
  }
}
console.log("ASN.1 序列（≥100B）总数:", keyHits, " ← 内嵌公钥/密钥表会在这里");
