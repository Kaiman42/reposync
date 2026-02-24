//go:build windows
package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

func getIconPath(name string) string {
	if name == "commit" {
		return "C:\\Windows\\System32\\imageres.dll,3"
	}

	execPath, _ := os.Executable()
	baseDir := filepath.Dir(execPath)
	
	p := filepath.Join(baseDir, "icons", name+".ico")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	// Tenta um nível acima se estiver rodando da pasta windows/
	p = filepath.Join(baseDir, "..", "windows", "icons", name+".ico")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	return "C:\\Windows\\System32\\imageres.dll,3"
}

var NameMap = map[string]string{
	"synced":       "synced",
	"commit":       "commit",
	"untracked":    "untracked",
	"pending_sync": "pending_sync",
	"no_remote":    "no_remote",
	"not_init":     "not_init",
}

func UpdateDirectoryIcon(path, status string) bool {
	iniPath := filepath.Join(path, "desktop.ini")
	fullIconPath := getIconPath(NameMap[status])
	absIconPath, _ := filepath.Abs(fullIconPath)

	// Usar aspas no caminho para evitar problemas com espaços e garantir compatibilidade
	newContent := fmt.Sprintf("\xef\xbb\xbf[.ShellClassInfo]\r\nIconResource=\"%s\",0\r\nIconIndex=0\r\n", absIconPath)

	oldBytes, err := os.ReadFile(iniPath)
	if err == nil && string(oldBytes) == newContent {
		return false
	}

	exec.Command("attrib", "-s", "-h", "-r", iniPath).Run()
	os.WriteFile(iniPath, []byte(newContent), 0644)
	exec.Command("attrib", "+s", "+h", iniPath).Run()
	
	// Resetar atributos da pasta e aplicar novamente para forçar o Windows a reler
	exec.Command("attrib", "-r", "-s", path).Run()
	exec.Command("attrib", "+r", "+s", path).Run()
	
	return true
}

func RefreshUI(paths []string) {
	if len(paths) == 0 {
		return
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shChangeNotify := shell32.NewProc("SHChangeNotify")
	
	for _, p := range paths {
		ptr, _ := syscall.UTF16PtrFromString(p)
		shChangeNotify.Call(0x00002000, 0x0005, uintptr(unsafe.Pointer(ptr)), 0)
	}
	
	// Refresh global é necessário quando mudamos de ícone custom para outro ícone custom
	shChangeNotify.Call(0x08000000, 0, 0, 0)
}
