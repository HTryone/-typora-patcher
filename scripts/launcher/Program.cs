// Typora Green 启动器（编译版；取代了早期 START-Typora.bat 里的内联 PowerShell 写法）
// 编译：见 scripts/build-launcher.sh（csc -target:winexe -codepage:65001）
// 行为：① 校验 app\Typora.exe 存在；② 启动前清理他机残留的试用数据（详见 CleanStaleTrial）；
//       ③ 首次运行建快捷方式（包根 + 桌面，桌面路径按注册表 User Shell Folders 解析，
//       兼容被重定向到 D:\desktop 这类情况）；④ 启动 Typora。
// 首次运行标记放在包外（%LOCALAPPDATA%\TyporaGreen.setup-done），包内残留的 .lnk 骗不过它。
//
// 【为什么清理做在 exe 里】.bat 依赖控制台代码页与 reg/powershell 的调用环境，换机器不靠谱；
// C# 编译产物行为处处一致。清理只删「试用相关」的键值与 Chromium 存储目录——不碰
// SLicense/IDate/MFp（本补丁的授权值），也不碰 themes/conf 等用户配置，
// 所以对官方正版共存场景安全（B 站反馈的「绿色版每次打开都提示激活」即旧试用残留所致）。
using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using Microsoft.Win32;

internal static class Program
{
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern int MessageBoxW(IntPtr hWnd, string text, string caption, uint type);

    private const uint MB_ICONERROR = 0x10;
    private const uint MB_ICONWARNING = 0x30;
    private const string MarkerName = "TyporaGreen.setup-done";

    [STAThread]
    private static int Main(string[] args)
    {
        string dir = AppDir();
        string appDir = Path.Combine(dir, "app");
        string exe = Path.Combine(appDir, "Typora.exe");

        if (!File.Exists(exe))
        {
            Msg("未在 app 文件夹里找到 Typora.exe。\n\n整个文件夹必须放在一起（别把文件挪出来），然后重试。",
                "Typora Green", MB_ICONERROR);
            return 1;
        }

        string marker = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), MarkerName);
        string pkgLnk = Path.Combine(dir, "Typora Green.lnk");
        string desktop = DesktopDir();
        string deskLnk = desktop == null ? null : Path.Combine(desktop, "Typora Green.lnk");

        // 标记是「三态」的：不存在 = 首次运行；内容 done = 已装（快捷方式丢了/文件夹挪了就自愈）；
        // 内容 removed = 用户在 SETUP.bat 里选过「Remove everything」，别再自动建回来。
        string markerText = "";
        try { if (File.Exists(marker)) markerText = File.ReadAllText(marker); } catch { }
        bool suppressed = markerText.IndexOf("removed", StringComparison.OrdinalIgnoreCase) >= 0;

        // 需要（重新）建快捷方式：首次运行；快捷方式被删了；文件夹被挪了位置（快捷方式指向旧路径）
        bool need = !suppressed
                    && (!File.Exists(marker)
                        || !ShortcutOk(pkgLnk, exe)
                        || (deskLnk != null && !ShortcutOk(deskLnk, exe)));

        if (need)
        {
            try
            {
                CreateShortcut(pkgLnk, exe, appDir);
                if (deskLnk != null)
                    CreateShortcut(deskLnk, exe, appDir);
            }
            catch (Exception ex)
            {
                Msg("创建快捷方式失败：" + ex.Message + "\n\n可以双击 SETUP.bat 手动创建。",
                    "Typora Green", MB_ICONWARNING);
            }
            try { File.WriteAllText(marker, "done"); } catch { }
        }

        CleanStaleTrialOnce();

        try
        {
            Process.Start(new ProcessStartInfo(exe)
            {
                WorkingDirectory = appDir,
                UseShellExecute = true
            });
        }
        catch (Exception ex)
        {
            Msg("启动 Typora 失败：" + ex.Message, "Typora Green", MB_ICONERROR);
            return 1;
        }
        return 0;
    }

    // ============================================================
    // 旧试用残留清理（检测到才清，不锁死）
    // ============================================================

    // 他机残留的试用数据会让绿色版首次启动就被顶回「激活/试用」态（B 站反馈）。
    // 每次启动先检测：注册表里出现试用键（TrialStart/TDate/FirstLaunch/LaunchCount，
    // 只有进入试用态 Typora 才会写）才算有问题 → 清掉试用键值与 Chromium 存储目录；
    // 没有就整体跳过，一个字节都不动。不用标记文件，_state 会自愈：
    // 残留被清掉后后续启动全部走「无问题」分支， Typora 哪天重写试用键也会再次被清。
    // 清理范围：试用相关键值与存储目录；SLicense/IDate/MFp（本补丁写入的授权值）
    // 与 themes/conf 等用户配置一律不动，对「官方正版共存」场景安全。
    // 本方法在启动 Typora 之前调用，不存在文件被占用的问题。
    private static void CleanStaleTrialOnce()
    {
        string[] trialValues = { "TrialStart", "TDate", "FirstLaunch", "LaunchCount" };

        // ① 检测：注册表里有没有试用键
        bool stale = false;
        try
        {
            using (RegistryKey k = Registry.CurrentUser.OpenSubKey(@"Software\Typora"))
            {
                if (k != null)
                {
                    foreach (string v in trialValues)
                    {
                        if (k.GetValue(v) != null) { stale = true; break; }
                    }
                }
            }
        }
        catch { return; }

        // 没问题就什么都不做
        if (!stale) return;

        // ② 有问题：清注册表试用键值（逐个删，绝不整键删除）
        try
        {
            using (RegistryKey k = Registry.CurrentUser.OpenSubKey(@"Software\Typora", true))
            {
                if (k != null)
                {
                    foreach (string v in trialValues)
                    {
                        try { k.DeleteValue(v, false); } catch { }
                    }
                }
            }
        }
        catch { }

        // ③ 清 Chromium 试用持久化数据（逐个目录删，不动 Typora 用户目录本身）
        try
        {
            string baseDir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Typora");
            if (Directory.Exists(baseDir))
            {
                string[] trialDirs = {
                    "Local Storage", "Session Storage", "SharedStorage", "DIPS",
                    "Code Cache", "Cache", "GPUCache", "DawnGraphiteCache",
                    "DawnWebGPUCache", "Shared Dictionary", "Network"
                };
                foreach (string d in trialDirs)
                {
                    try { Directory.Delete(Path.Combine(baseDir, d), true); } catch { }
                }
            }
        }
        catch { }
    }

    private static string AppDir()
    {
        try
        {
            string p = Assembly.GetExecutingAssembly().Location;
            if (!string.IsNullOrEmpty(p))
            {
                string d = Path.GetDirectoryName(p);
                if (!string.IsNullOrEmpty(d)) return d;
            }
        }
        catch { }
        return AppDomain.CurrentDomain.BaseDirectory;
    }

    private static string DesktopDir()
    {
        try
        {
            using (RegistryKey k = Registry.CurrentUser.OpenSubKey(
                @"Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders"))
            {
                if (k != null)
                {
                    string v = k.GetValue("Desktop") as string;
                    if (!string.IsNullOrEmpty(v))
                    {
                        string p = Environment.ExpandEnvironmentVariables(v);
                        if (Directory.Exists(p)) return p;
                    }
                }
            }
        }
        catch { }
        string d = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        return string.IsNullOrEmpty(d) ? null : d;
    }

    // 判断已有快捷方式是否还指向当前这个包（文件不存在、指向旧路径都算“要重建”）
    private static bool ShortcutOk(string lnkPath, string target)
    {
        try
        {
            if (!File.Exists(lnkPath)) return false;
            Type shellType = Type.GetTypeFromProgID("WScript.Shell");
            if (shellType == null) return true;
            object shell = Activator.CreateInstance(shellType);
            object lnk = shellType.InvokeMember("CreateShortcut", BindingFlags.InvokeMethod, null, shell,
                new object[] { lnkPath });
            object t = lnk.GetType().InvokeMember("TargetPath", BindingFlags.GetProperty, null, lnk, null);
            string s = t as string;
            return !string.IsNullOrEmpty(s) && string.Equals(s, target, StringComparison.OrdinalIgnoreCase);
        }
        catch
        {
            return true; // 判断不了就当“没问题”，别每次启动都重建
        }
    }

    // 用 WScript.Shell 的 COM 建 .lnk（不依赖 PowerShell，也不受执行策略影响）
    private static void CreateShortcut(string lnkPath, string target, string workDir)
    {
        Type shellType = Type.GetTypeFromProgID("WScript.Shell");
        if (shellType == null) throw new InvalidOperationException("WScript.Shell 不可用");
        object shell = Activator.CreateInstance(shellType);
        object lnk = shellType.InvokeMember("CreateShortcut", BindingFlags.InvokeMethod, null, shell,
            new object[] { lnkPath });
        Type lnkType = lnk.GetType();
        lnkType.InvokeMember("TargetPath", BindingFlags.SetProperty, null, lnk, new object[] { target });
        lnkType.InvokeMember("WorkingDirectory", BindingFlags.SetProperty, null, lnk, new object[] { workDir });
        lnkType.InvokeMember("IconLocation", BindingFlags.SetProperty, null, lnk, new object[] { target + ",0" });
        lnkType.InvokeMember("Save", BindingFlags.InvokeMethod, null, lnk, null);
    }

    private static void Msg(string text, string caption, uint type)
    {
        MessageBoxW(IntPtr.Zero, text, caption, type);
    }
}
