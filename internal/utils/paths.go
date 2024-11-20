package utils

import (
	"os"
	"path/filepath"
)

func GetAppDataDir() string {
	appData, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(GetRootDir(), "data")
	}
	return filepath.Join(appData, "x-script")
}

func GetRootDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(exe))
}

func GetAssetPath(name string) string {
	return filepath.Join(GetRootDir(), "assets", name)
}
