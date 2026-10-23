#!/usr/bin/env bash
# 编译 Typora Green 启动器 -> green-test/START-Typora.exe
# 用系统自带 .NET Framework 4.x 的 csc.exe，产出十几 KB、零额外依赖的 exe。
# 用法：bash scripts/build-launcher.sh
set -euo pipefail

WS="$(cd "$(dirname "$0")/.." && pwd)"
CSC="/c/Windows/Microsoft.NET/Framework64/v4.0.30319/csc.exe"
SRC="$WS/scripts/launcher/Program.cs"
OUT="$WS/green-test/START-Typora.exe"

[ -x "$CSC" ] || { echo "找不到 csc.exe: $CSC" >&2; exit 1; }
[ -f "$SRC" ] || { echo "找不到源码: $SRC" >&2; exit 1; }

# csc 是 Windows 程序，路径转成 Windows 形式
SRC_W="$(cygpath -w "$SRC")"
OUT_W="$(cygpath -w "$OUT")"

"$CSC" -nologo -target:winexe -optimize+ -platform:anycpu -codepage:65001 \
  -out:"$OUT_W" -r:System.dll -r:System.Core.dll "$SRC_W"

ls -l "$OUT"
echo "编译完成：$OUT"
