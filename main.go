package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	config Config
)

type RepoStats struct {
	LastChange string `json:"last_change"`
	Size       string `json:"size"`
	Status     string `json:"status"`
}

func init() {
	config = loadConfig()
}

func getGitStatus(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		return "not_init"
	}

	// git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = getSysProcAttr()
	}
	output, _ := cmd.Output()

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	hasUntracked := false
	hasModified := false

	for _, line := range lines {
		if line == "" {
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
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = getSysProcAttr()
	}
	err := cmd.Run()
	if err != nil {
		// Não tem upstream. Verifica se tem qualquer remoto.
		remoteCmd := exec.Command("git", "remote")
		remoteCmd.Dir = path
		if runtime.GOOS == "windows" {
			remoteCmd.SysProcAttr = getSysProcAttr()
		}
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
	if runtime.GOOS == "windows" {
		aheadCmd.SysProcAttr = getSysProcAttr()
	}
	aheadOut, _ := aheadCmd.Output()
	ahead := strings.TrimSpace(string(aheadOut))

	behindCmd := exec.Command("git", "rev-list", "HEAD..@{u}", "--count")
	behindCmd.Dir = path
	if runtime.GOOS == "windows" {
		behindCmd.SysProcAttr = getSysProcAttr()
	}
	behindOut, _ := behindCmd.Output()
	behind := strings.TrimSpace(string(behindOut))

	if ahead != "0" || behind != "0" {
		return "pending_sync" // Roxo
	}

	return "synced" // Verde
}

func updateRepo(repoPath string, quiet bool) {
	status := getGitStatus(repoPath)
	if !quiet {
		fmt.Printf("%s: %s\n", repoPath, status)
	}
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

	if flag.NArg() < 1 {
		// Abre o Wails Dashboard nativo por padrão (necessário para o "wails build" e uso como App Desktop)
		startDashboard()
		return
	}

	command := flag.Arg(0)

	switch command {
	case "run":
		syncAll(*quiet)
	case "watch":
		startWatcher(config.BasePaths)
	case "dashboard":
		startDashboard()
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

	// Se estiver rodando um executável solto ou pelo 'wails dev', vamos forçar
	// apontar para a versão oficial que o Wails gera na pasta build/bin/ se ela existir
	projectName := filepath.Base(exePath)
	wd, _ := os.Getwd()
	wailsBinPath := filepath.Join(wd, "build", "bin", projectName)
	if _, err := os.Stat(wailsBinPath); err == nil && !strings.Contains(filepath.ToSlash(exePath), "build/bin") {
		exePath = wailsBinPath
		fmt.Println("Usando o executável oficial do Wails em:", exePath)
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

	vbsContent := fmt.Sprintf("Set ws = CreateObject(\"WScript.Shell\")\n"+
		"Set shortcut = ws.CreateShortcut(\"%s\")\n"+
		"shortcut.TargetPath = \"%s\"\n"+
		"shortcut.Arguments = \"dashboard\"\n"+
		"shortcut.WorkingDirectory = \"%s\"\n"+
		"shortcut.IconLocation = \"%s,0\"\n"+
		"shortcut.Save\n", shortcutPath, exePath, filepath.Dir(exePath), exePath)

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
