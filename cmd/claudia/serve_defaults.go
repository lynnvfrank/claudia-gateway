package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func defaultSupervisorBifrostBin() string {
	dir := executableDir()
	if dir == "" {
		return "bifrost"
	}
	names := []string{"bifrost-http", "bifrost"}
	if runtime.GOOS == "windows" {
		names = []string{"bifrost-http.exe", "bifrost.exe", "bifrost-http", "bifrost"}
	}
	if p := firstExistingFile(dir, names); p != "" {
		return p
	}
	return "bifrost"
}

func defaultSupervisorQdrantBin() string {
	dir := executableDir()
	if dir == "" {
		return ""
	}
	names := []string{"qdrant"}
	if runtime.GOOS == "windows" {
		names = []string{"qdrant.exe", "qdrant"}
	}
	return firstExistingFile(dir, names)
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func firstExistingFile(dir string, names []string) string {
	for _, n := range names {
		p := filepath.Join(dir, n)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
