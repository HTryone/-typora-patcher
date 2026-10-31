package system

import "os"

// osExecutable / osArgs / workingDir 从 main 包挪进来，避免 system 反向依赖。
func osExecutable() (string, error) { return os.Executable() }
func osArgs() []string              { return os.Args[1:] }
func workingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
