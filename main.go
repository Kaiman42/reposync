package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

var (
	config    Config
	repoState = make(map[string]*RepoStats)
)

type RepoStats struct {
	LastChange string `json:"last_change"`
	Size       string `json:"size"`
	Status     string `json:"status"`
}

func init() {
	config = loadConfig()
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func getGitStatus(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		return "not_init"
	}

	// git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
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
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	err := cmd.Run()
	if err != nil {
		// Não tem upstream. Verifica se tem qualquer remoto.
		remoteCmd := exec.Command("git", "remote")
		remoteCmd.Dir = path
		if runtime.GOOS == "windows" {
			remoteCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
		aheadCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	aheadOut, _ := aheadCmd.Output()
	ahead := strings.TrimSpace(string(aheadOut))

	behindCmd := exec.Command("git", "rev-list", "HEAD..@{u}", "--count")
	behindCmd.Dir = path
	if runtime.GOOS == "windows" {
		behindCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
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

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
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
