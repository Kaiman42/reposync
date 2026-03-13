package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/Kaiman42/reposync/internal/config"
	"github.com/Kaiman42/reposync/internal/git"
	"github.com/Kaiman42/reposync/internal/ui"
	"github.com/Kaiman42/reposync/internal/watcher"
)

//go:embed all:frontend
var assets embed.FS

//go:embed build/linux/reposync.svg
var faviconSVG []byte

//go:embed build/windows/reposync.ico
var iconICO []byte

var (
	cfg config.Config
)

func updateRepo(repoPath string, quiet bool) bool {
	status := git.GetGitStatus(repoPath)
	if !quiet {
		fmt.Printf("%s: %s\n", repoPath, status)
	}
	return ui.UpdateDirectoryIcon(repoPath, status)
}

func syncAll(quiet bool) {
	repos := git.FindRepos(cfg.BasePaths)
	for _, repo := range repos {
		updateRepo(repo, quiet)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("PANIC FATAL: %v", r)
			ui.ShowMessage("RepoSync - Erro Fatal", errStr)
			os.Exit(1)
		}
	}()

	cfg = config.LoadConfig()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: reposync <command> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  run           Run sync once and exit\n")
		fmt.Fprintf(os.Stderr, "  watch         Start watcher daemon\n")
		fmt.Fprintf(os.Stderr, "  install-hooks Install git hooks in all repos\n")
		fmt.Fprintf(os.Stderr, "  setup         Initial setup (hooks + run + watch)\n")
		fmt.Fprintf(os.Stderr, "  create-shortcut Create a desktop shortcut automatically\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	quiet := flag.Bool("q", false, "Quiet mode")
	flag.Parse()

	command := "dashboard"
	if flag.NArg() >= 1 {
		command = flag.Arg(0)
	}

	switch command {
	case "run":
		syncAll(*quiet)
	case "watch":
		watcher.StartWatcher(cfg.BasePaths)
	case "dashboard":
		go func() {
			defer func() { recover() }()
			watcher.StartWatcher(cfg.BasePaths)
		}()
		startDashboardGUI()
	case "install-hooks":
		git.InstallHooksAll(cfg.BasePaths)
	case "setup":
		createShortcut()
		git.InstallHooksAll(cfg.BasePaths)
		syncAll(*quiet)
		fmt.Println("\n[OK] Repositories initialized. Starting watcher...")
		watcher.StartWatcher(cfg.BasePaths)
	case "create-shortcut":
		createShortcut()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func startDashboardGUI() {
	app := NewApp()

	assetsRoot, err := fs.Sub(assets, "frontend")
	if err != nil {
		fatalError("Erro ao acessar arquivos internos: " + err.Error())
		return
	}

	err = wails.Run(&options.App{
		Title:         "RepoSync Dashboard",
		Width:         1024,
		Height:        768,
		DisableResize: true,
		Frameless:     true,
		AssetServer: &assetserver.Options{
			Assets: assetsRoot,
		},
		BackgroundColour: &options.RGBA{R: 5, G: 7, B: 10, A: 0},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			BackdropType:                      windows.None,
			DisableWindowIcon:                 true,
			DisableFramelessWindowDecorations: true,
			Theme:                             windows.Dark,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: true,
		},
	})
	if err != nil {
		fatalError("Erro ao iniciar Dashboard: " + err.Error())
	}
}

func fatalError(msg string) {
	fmt.Println(msg)
	ui.ShowMessage("RepoSync - Erro", msg)
}

func getFolderSize(path string) string {
	var size int64
	count := 0
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || count > 1000 {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			size += info.Size()
		}
		count++
		return nil
	})
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
}

func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	if diff.Hours() > 24 {
		return fmt.Sprintf("%.0f dias atrás", diff.Hours()/24)
	}
	if diff.Hours() >= 1 {
		return fmt.Sprintf("%.0f horas atrás", diff.Hours())
	}
	if diff.Minutes() >= 1 {
		return fmt.Sprintf("%.0f min atrás", diff.Minutes())
	}
	return "Agora mesmo"
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		cmd.SysProcAttr = ui.GetSysProcAttr()
		err = cmd.Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Println("Erro ao abrir navegador:", err)
	}
}

func createShortcut() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("Erro ao obter caminho do executável:", err)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	if !strings.Contains(filepath.ToSlash(exePath), "build/bin") {
		wd, _ := os.Getwd()
		officialName := "reposync"
		if runtime.GOOS == "windows" {
			officialName += ".exe"
		}
		target := filepath.Join(wd, "build", "bin", officialName)
		if _, err := os.Stat(target); err == nil {
			exePath = target
			fmt.Println("Usando o executável oficial encontrado em:", exePath)
		}
	}

	switch runtime.GOOS {
	case "linux":
		createLinuxShortcut(exePath)
	case "windows":
		createWindowsShortcut(exePath)
	default:
		fmt.Println("Sistema não suportado para criar atalho automaticamente.")
	}
}

func createLinuxShortcut(exePath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Erro ao obter diretório do usuário:", err)
		return
	}

	iconsDir := filepath.Join(home, ".local", "share", "icons")
	os.MkdirAll(iconsDir, 0755)
	iconPath := filepath.Join(iconsDir, "reposync.svg")

	err = os.WriteFile(iconPath, faviconSVG, 0644)
	if err != nil {
		fmt.Println("Aviso: Erro ao salvar ícone oficial localmente:", err)
		iconPath = "reposync"
	} else {
		fmt.Println("Ícone extraído e instalado em:", iconPath)
	}

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Name=Reposync
Comment=Ferramenta de Sincronização de Repositórios
Exec=%s dashboard
Icon=%s
Terminal=false
Type=Application
Categories=Development;Utility;
StartupNotify=true
`, exePath, iconPath)

	var desktopDir string
	cmd := exec.Command("xdg-user-dir", "DESKTOP")
	out, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		desktopDir = strings.TrimSpace(string(out))
	} else {
		desktopDir = filepath.Join(home, "Área de trabalho")
		if _, err := os.Stat(desktopDir); os.IsNotExist(err) {
			desktopDir = filepath.Join(home, "Desktop")
		}
	}

	shortcutPath := filepath.Join(desktopDir, "Reposync.desktop")
	err = os.WriteFile(shortcutPath, []byte(desktopContent), 0755)
	if err != nil {
		fmt.Println("Erro ao criar atalho na área de trabalho:", err)
	} else {
		fmt.Println("Atalho criado com sucesso na área de trabalho:", shortcutPath)
		exec.Command("chmod", "+x", shortcutPath).Run()
	}

	appsDir := filepath.Join(home, ".local", "share", "applications")
	os.MkdirAll(appsDir, 0755)
	appShortcutPath := filepath.Join(appsDir, "reposync.desktop")
	err = os.WriteFile(appShortcutPath, []byte(desktopContent), 0755)
	if err == nil {
		fmt.Println("Atalho criado no menu de aplicativos:", appShortcutPath)
	}
}

func createWindowsShortcut(exePath string) {
	home, _ := os.UserHomeDir()
	desktopDir := filepath.Join(home, "Desktop")
	shortcutPath := filepath.Join(desktopDir, "Reposync.lnk")

	iconPath := exePath
	configDir, err := os.UserConfigDir()
	if err == nil {
		appDir := filepath.Join(configDir, "reposync")
		os.MkdirAll(appDir, 0755)
		localIconPath := filepath.Join(appDir, "reposync_v2.ico")
		
		// Remove old icon to help refresh cache
		os.Remove(filepath.Join(appDir, "reposync.ico"))
		
		err = os.WriteFile(localIconPath, iconICO, 0644)
		if err == nil {
			iconPath = localIconPath
			fmt.Println("Ícone oficial extraído para:", iconPath)
		}
	}

	vbsContent := fmt.Sprintf("Set ws = CreateObject(\"WScript.Shell\")\n"+
		"Set shortcut = ws.CreateShortcut(\"%s\")\n"+
		"shortcut.TargetPath = \"%s\"\n"+
		"shortcut.Arguments = \"dashboard\"\n"+
		"shortcut.WorkingDirectory = \"%s\"\n"+
		"shortcut.IconLocation = \"%s\"\n"+
		"shortcut.Save\n", shortcutPath, exePath, filepath.Dir(exePath), iconPath)

	vbsPath := filepath.Join(os.TempDir(), "create_shortcut.vbs")
	os.WriteFile(vbsPath, []byte(vbsContent), 0644)
	defer os.Remove(vbsPath)

	cmd := exec.Command("wscript", vbsPath)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = ui.GetSysProcAttr()
	}
	if err := cmd.Run(); err != nil {
		fmt.Println("Erro ao criar atalho no Windows:", err)
	} else {
		fmt.Println("Atalho criado com sucesso na Área de Trabalho:", shortcutPath)
	}
}
