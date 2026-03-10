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
)

//go:embed all:frontend
var assets embed.FS

//go:embed linux/reposync.svg
var faviconSVG []byte

//go:embed icons/Reposync.ico
var iconICO []byte

var (
	config Config
)

func getGitStatus(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		return "not_init"
	}

	// git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	cmd.SysProcAttr = getSysProcAttr()
	output, _ := cmd.Output()

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	hasUntracked := false
	hasModified := false

	for _, line := range lines {
		if line == "" {
			continue
		}

		// O formato porcelain é "XY caminho", onde XY são os estados
		// Ignora arquivos de sistema
		if strings.Contains(line, "desktop.ini") || strings.Contains(line, ".directory") {
			continue
		}

		statusPart := line[:2]
		if statusPart == "??" {
			hasUntracked = true
		} else {
			// Se tem qualquer coisa nas colunas X ou Y que não seja espaço ou ?, é porque houve mudança
			hasModified = true
		}
	}

	// PRIORIDADE 1: Arquivos não rastreados (Vermelho)
	if hasUntracked {
		return "untracked"
	}
	// PRIORIDADE 2: Mudanças pendentes (Amarelo)
	if hasModified {
		return "commit"
	}

	// PRIORIDADE 3: Sincronismo com Remoto
	return getSyncStatus(repoPath)
}

func getSyncStatus(path string) string {
	// Verifica se tem upstream
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = path
	cmd.SysProcAttr = getSysProcAttr()
	err := cmd.Run()
	if err != nil {
		// Não tem upstream. Verifica se tem qualquer remoto.
		remoteCmd := exec.Command("git", "remote")
		remoteCmd.Dir = path
		remoteCmd.SysProcAttr = getSysProcAttr()
		remoteOut, _ := remoteCmd.Output()
		if strings.TrimSpace(string(remoteOut)) == "" {
			// Sem nenhum remoto: Verde (ou use "no_remote" se preferir Laranja)
			return "no_remote"
		}
		// Tem remoto mas não está trackeando nada: Roxo (precisa de push inicial)
		return "pending_sync"
	}

	// Verifica à frente (Ahead) ou atrás (Behind)
	// Precisaria de git fetch para o behind, mas o Ahead é instantâneo
	aheadCmd := exec.Command("git", "rev-list", "@{u}..HEAD", "--count")
	aheadCmd.Dir = path
	aheadCmd.SysProcAttr = getSysProcAttr()
	aheadOut, _ := aheadCmd.Output()
	ahead := strings.TrimSpace(string(aheadOut))

	behindCmd := exec.Command("git", "rev-list", "HEAD..@{u}", "--count")
	behindCmd.Dir = path
	behindCmd.SysProcAttr = getSysProcAttr()
	behindOut, _ := behindCmd.Output()
	behind := strings.TrimSpace(string(behindOut))

	if ahead != "0" || behind != "0" {
		return "pending_sync" // Roxo
	}

	return "synced" // Verde
}

func updateRepo(repoPath string, quiet bool) bool {
	status := getGitStatus(repoPath)
	if !quiet {
		fmt.Printf("%s: %s\n", repoPath, status)
	}
	return updateDirectoryIcon(repoPath, status)
}

func syncAll(quiet bool) {
	repos := findRepos(config.BasePaths)
	for _, repo := range repos {
		updateRepo(repo, quiet)
	}
}

func findRepos(bases []string) []string {
	var repos []string
	seen := make(map[string]bool)
	for _, base := range bases {
		files, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				path := filepath.Join(base, f.Name())
				if !seen[path] {
					repos = append(repos, path)
					seen[path] = true
				}
			}
		}
	}
	return repos
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("PANIC FATAL: %v", r)
			showMessage("RepoSync - Erro Fatal", errStr)
			os.Exit(1)
		}
	}()

	// Carrega a config aqui dentro para ser capturado pelo recover
	config = loadConfig()

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
		startWatcher(config.BasePaths)
	case "dashboard":
		go func() {
			defer func() { recover() }() // Silencia erros na thread do watcher para não matar o app
			startWatcher(config.BasePaths)
		}()
		startDashboardGUI()
	case "install-hooks":
		installHooksAll(config.BasePaths)
	case "setup":
		createShortcut()
		installHooksAll(config.BasePaths)
		syncAll(*quiet)
		fmt.Println("\n[OK] Repositories initialized. Starting watcher...")
		startWatcher(config.BasePaths)
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

	// Garante que o Wails veja o conteúdo da pasta frontend como a raiz do app
	assetsRoot, err := fs.Sub(assets, "frontend")
	if err != nil {
		fatalError("Erro ao acessar arquivos internos: " + err.Error())
		return
	}

	err = wails.Run(&options.App{
		Title:  "RepoSync Dashboard",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assetsRoot,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fatalError("Erro ao iniciar Dashboard: " + err.Error())
	}
}

func fatalError(msg string) {
	fmt.Println(msg)
	showMessage("RepoSync - Erro", msg)
}

func getRemoteURL(path string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = path
	cmd.SysProcAttr = getSysProcAttr()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if strings.HasPrefix(url, "git@") {
		url = strings.Replace(url, ":", "/", 1)
		url = strings.Replace(url, "git@", "https://", 1)
		url = strings.TrimSuffix(url, ".git")
	}
	return url
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

func getRepoLastMod(path string) time.Time {
	cmd := exec.Command("git", "log", "-1", "--format=%ct")
	cmd.Dir = path
	cmd.SysProcAttr = getSysProcAttr()
	out, err := cmd.Output()
	if err == nil {
		var sec int64
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &sec)
		return time.Unix(sec, 0)
	}
	info, _ := os.Stat(path)
	if info != nil {
		return info.ModTime()
	}
	return time.Now()
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
		cmd.SysProcAttr = getSysProcAttr()
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

func installHooksAll(bases []string) {
	repos := findRepos(bases)
	self, _ := os.Executable()
	for _, repo := range repos {
		installHook(repo, self)
	}
	fmt.Printf("Hooks installed in %d repositories.\n", len(repos))
}

func installHook(repoPath, selfPath string) {
	hooksDir := filepath.Join(repoPath, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	hookNames := []string{"post-commit", "post-merge", "post-checkout", "post-rewrite", "post-applypatch", "post-reset", "post-update", "post-switch"}

	// Convert path for bash (Windows compatibility)
	selfPath = strings.ReplaceAll(selfPath, "\\", "/")

	content := fmt.Sprintf("#!/bin/bash\n# Auto-generated by RepoSync\n\"%s\" run -q >/dev/null 2>&1 &\nexit 0\n", selfPath)

	for _, name := range hookNames {
		hookPath := filepath.Join(hooksDir, name)
		os.WriteFile(hookPath, []byte(content), 0755)
	}
}

func createShortcut() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("Erro ao obter caminho do executável:", err)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	// Se estiver rodando em desenvolvimento ou via 'go run', tenta encontrar o executável oficial
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

	// Extrair e salvar o ícone SVG embutido
	iconsDir := filepath.Join(home, ".local", "share", "icons")
	os.MkdirAll(iconsDir, 0755)
	iconPath := filepath.Join(iconsDir, "reposync.svg")

	err = os.WriteFile(iconPath, faviconSVG, 0644)
	if err != nil {
		fmt.Println("Aviso: Erro ao salvar ícone oficial localmente:", err)
		iconPath = "reposync" // Tenta deixar o sistema descobrir sozinho caso falhe
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

	// Tenta extrair o ícone oficial para um local persistente
	iconPath := exePath // Fallback para o ícone embutido no .exe
	configDir, err := os.UserConfigDir()
	if err == nil {
		appDir := filepath.Join(configDir, "reposync")
		os.MkdirAll(appDir, 0755)
		localIconPath := filepath.Join(appDir, "reposync.ico")
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
		cmd.SysProcAttr = getSysProcAttr()
	}
	if err := cmd.Run(); err != nil {
		fmt.Println("Erro ao criar atalho no Windows:", err)
	} else {
		fmt.Println("Atalho criado com sucesso na Área de Trabalho:", shortcutPath)
	}
}
