#!/usr/bin/env bash
# 编译 Typora Green 启动器 -> green-test/START-Typora.exe
# 用系统自带 .NET Framework 4.x 的 csc.exe，产出十几 KB、零额外依赖的 exe。
# 用法：bash scripts/build-launcher.sh
set -euo pipefail

WS="$(cd "$(dirname "$0")/.." && pwd)"
CSC="/c/Windows/Microsoft.NET/Framework64/v4.0.30319/csc.exe"
SRC="$WS/scripts/launcher/Program.cs"
OUT="$WS/green-test/START-Typora.exe"
# 启动器图标：Typora 原版 app.ico（9 尺寸）。包内 app\resources\assets\app.ico 在打包时才有，
# 构建时从绿色包暂存目录取，没有就退回本机 Typora 安装目录，再没有就无图标编译。
ICON_SRC=""
for c in "$WS/tmp/green-local/app/resources/assets/app.ico" "/d/Apps/Typora/resources/assets/app.ico"; do
  [ -f "$c" ] && { ICON_SRC="$c"; break; }
done
ICON_ARG=""
[ -n "$ICON_SRC" ] && ICON_ARG="-win32icon:$(cygpath -w "$ICON_SRC")"

[ -x "$CSC" ] || { echo "找不到 csc.exe: $CSC" >&2; exit 1; }
[ -f "$SRC" ] || { echo "找不到源码: $SRC" >&2; exit 1; }

# csc 是 Windows 程序，路径转成 Windows 形式
SRC_W="$(cygpath -w "$SRC")"
OUT_W="$(cygpath -w "$OUT")"

"$CSC" -nologo -target:winexe -optimize+ -platform:anycpu -codepage:65001 \
  -out:"$OUT_W" -r:System.dll -r:System.Core.dll $ICON_ARG "$SRC_W"

ls -l "$OUT"
echo "编译完成：$OUT"
