package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

//go:embed index.html style.css script.js
var dashboardFs embed.FS

func killExistingInstances() {
	if runtime.GOOS == "windows" {
		currentPid := os.Getpid()
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq reposync.exe", "/FO", "CSV", "/NH")
		cmd.SysProcAttr = getSysProcAttr()
		output, _ := cmd.Output()
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			parts := strings.Split(line, ",")
			if len(parts) > 1 {
				pidStr := strings.Trim(parts[1], "\"")
				var pid int
				fmt.Sscanf(pidStr, "%d", &pid)
				if pid != 0 && pid != currentPid {
					kill := exec.Command("taskkill", "/F", "/PID", pidStr)
					kill.SysProcAttr = getSysProcAttr()
					kill.Run()
				}
			}
		}
	}
}

func startDashboard() {
	killExistingInstances()
	http.Handle("/", http.FileServer(http.FS(dashboardFs)))
	http.HandleFunc("/api/repos", getRepos)
	http.HandleFunc("/api/open", handleOpenAction)
	http.HandleFunc("/api/add-path", handleAddPath)
	http.HandleFunc("/api/remove-path", handleRemovePath)
	http.HandleFunc("/api/config", getBasePaths)
	http.HandleFunc("/api/repo-details", getRepoDetails)

	port := ":8888"
	url := "http://localhost" + port

	fmt.Println("Dashboard ativo em:", url)
	fmt.Println("Pressione Ctrl+C para encerrar...")

	go openAppWindow(url)

	server := &http.Server{Addr: port}

	// Canal para sinais de interrupção
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Erro no servidor: %v\n", err)
		}
	}()

	<-stop // Aguarda Ctrl+C
	fmt.Println("\nEncerrando servidor...")
}

func getRepos(w http.ResponseWriter, r *http.Request) {
	repos := findRepos(config.BasePaths)
	type RepoInfo struct {
		Path       string `json:"path"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Size       string `json:"size"`
		LastChange string `json:"last_change"`
		Relative   string `json:"relative_time"`
		RemoteURL  string `json:"remote_url"`
	}

	var list []RepoInfo
	for _, p := range repos {
		status := getGitStatus(p)
		modTime := getRepoLastMod(p)
		list = append(list, RepoInfo{
			Path:       p,
			Name:       filepath.Base(p),
			Status:     status,
			Size:       getFolderSize(p),
			LastChange: modTime.Format("02/01/2006 15:04"),
			Relative:   formatRelativeTime(modTime),
			RemoteURL:  getRemoteURL(p),
		})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	json.NewEncoder(w).Encode(list)
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var newCfg Config
		json.NewDecoder(r.Body).Decode(&newCfg)
		config = newCfg
		saveConfig(config)
	}
	json.NewEncoder(w).Encode(config)
}

func handleOpenAction(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	action := r.URL.Query().Get("action")

	switch action {
	case "explorer":
		if runtime.GOOS == "windows" {
			exec.Command("explorer", path).Run()
		} else {
			exec.Command("xdg-open", path).Run()
		}
	case "code":
		cmd := exec.Command("code", path)
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = getSysProcAttr()
		}
		cmd.Run()
	case "remote":
		url := r.URL.Query().Get("url")
		if url != "" {
			openBrowser(url)
		}
	}
}

func getBasePaths(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(config.BasePaths)
}

func handleRemovePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	newPaths := []string{}
	for _, p := range config.BasePaths {
		if !strings.EqualFold(p, req.Path) {
			newPaths = append(newPaths, p)
		}
	}
	config.BasePaths = newPaths
	saveConfig(config)
	w.WriteHeader(http.StatusOK)
}

func handleAddPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Path != "" {
		exists := false
		for _, p := range config.BasePaths {
			if strings.EqualFold(p, req.Path) {
				exists = true
				break
			}
		}
		if !exists {
			config.BasePaths = append(config.BasePaths, req.Path)
			saveConfig(config)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func getRemoteURL(path string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = path
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = getSysProcAttr()
	}
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
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = getSysProcAttr()
	}
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

func openAppWindow(url string) {
	if runtime.GOOS == "windows" {
		// Tenta abrir o Edge em modo app sem usar cmd /c
		cmd := exec.Command("msedge", "--app="+url)
		cmd.SysProcAttr = getSysProcAttr()
		err := cmd.Start()
		if err != nil {
			// Fallback se msedge não estiver no PATH
			cmdFallback := exec.Command("cmd", "/c", "start", "msedge", "--app="+url)
			cmdFallback.SysProcAttr = getSysProcAttr()
			cmdFallback.Run()
		}
	} else {
		// Abre em modo app (janela limpa, sem abas)
		browsers := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"}
		for _, browser := range browsers {
			if _, err := exec.LookPath(browser); err == nil {
				exec.Command(browser, "--app="+url).Start()
				return
			}
		}
		fmt.Println("Nenhum navegador compatível encontrado (Chrome, Chromium ou Edge).")
	}
}

func getRepoDetails(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	details := make(map[string]interface{})
	details["path"] = path

	// Helper to run git commands
	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = getSysProcAttr()
		}
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}

	details["branch"] = runGit("rev-parse", "--abbrev-ref", "HEAD")
	details["last_commit"] = runGit("log", "-1", "--format=%B")
	details["last_author"] = runGit("log", "-1", "--format=%an <%ae>")
	details["last_date"] = runGit("log", "-1", "--format=%ai")
	details["commit_count"] = runGit("rev-list", "--count", "HEAD")
	details["remote_url"] = getRemoteURL(path)
	details["tags"] = runGit("tag")
	details["stashes"] = runGit("stash", "list")

	// Complex stats
	details["summary"] = runGit("shortlog", "-sn", "--all")
	details["recent_activity"] = runGit("log", "-5", "--oneline")

	// File stats
	details["disk_usage"] = runGit("count-objects", "-vH")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}
