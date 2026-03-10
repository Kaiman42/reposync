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
	var updated []string
	for _, repo := range repos {
		if updateRepo(repo, quiet) {
			updated = append(updated, repo)
		}
	}
	refreshUI(updated)
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
		installHooksAll(config.BasePaths)
		syncAll(*quiet)
		fmt.Println("\n[OK] Repositories initialized. Starting watcher...")
		startWatcher(config.BasePaths)
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
