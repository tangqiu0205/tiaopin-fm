package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// Portable packages opt in explicitly; the working directory is irrelevant.
func configurePortable() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	dir, err := portableDataDir(exe)
	if err != nil || dir == "" {
		return false, err
	}
	mygo.App.SetPath(mygo.PathUserData, dir)
	mygo.App.SetPath(mygo.PathCache, filepath.Join(dir, "cache"))
	mygo.App.SetPath(mygo.PathLogs, filepath.Join(dir, "logs"))
	return true, nil
}

func portableDataDir(exe string) (string, error) {
	root := filepath.Dir(exe)
	marker, err := os.Stat(filepath.Join(root, "portable.flag"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("无法读取便携模式标记：%w", err)
	}
	if !marker.Mode().IsRegular() {
		return "", errors.New("portable.flag 必须是普通文件")
	}
	dir := filepath.Join(root, "data")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("无法创建便携数据目录 %s：%w", dir, err)
	}
	// Fail at startup rather than silently using AppData or losing changes.
	probe, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return "", fmt.Errorf("便携数据目录不可写 %s：%w", dir, err)
	}
	closeErr := probe.Close()
	removeErr := os.Remove(probe.Name())
	if err := errors.Join(closeErr, removeErr); err != nil {
		return "", fmt.Errorf("无法完成便携数据目录写入检查 %s：%w", dir, err)
	}
	return dir, nil
}
