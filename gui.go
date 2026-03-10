package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend
var frontendFiles embed.FS

//go:embed linux/reposync.svg
var faviconSVG []byte

// App struct para o Wails
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// apiHandler roteia as requisições /api/* e serve o favicon
type apiHandler struct{}

func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/api/repos":
		getRepos(w, r)
	case "/api/open":
		handleOpenAction(w, r)
	case "/api/add-path":
		handleAddPath(w, r)
	case "/api/remove-path":
		handleRemovePath(w, r)
	case "/api/config":
		getBasePaths(w, r)
	case "/api/repo-details":
		getRepoDetails(w, r)
	case "/favicon.svg":
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(faviconSVG)
	default:
		http.NotFound(w, r)
	}
}

func startDashboard() {
	assets, _ := fs.Sub(frontendFiles, "frontend")
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "RepoSync Alpha — v2.7.0",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: &apiHandler{},
		},
		OnStartup: app.startup,
		Bind:      []interface{}{app},
	})
	if err != nil {
		fmt.Printf("Erro ao iniciar RepoSync: %v\n", err)
	}
}

func getRepos(w http.ResponseWriter, _ *http.Request) {
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

func handleOpenAction(_ http.ResponseWriter, r *http.Request) {
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
	case "open_with":
		customArgs := strings.Fields(r.URL.Query().Get("custom"))
		if len(customArgs) > 0 {
			cmdName := customArgs[0]
			args := append(customArgs[1:], path)
			cmd := exec.Command(cmdName, args...)
			if runtime.GOOS == "windows" {
				cmd.SysProcAttr = getSysProcAttr()
			}
			cmd.Start()
		}
	case "remote":
		url := r.URL.Query().Get("url")
		if url != "" {
			openBrowser(url)
		}
	}
}

func getBasePaths(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(config.BasePaths)
}

func handleRemovePath(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")

	newPaths := []string{}
	for _, p := range config.BasePaths {
		if !strings.EqualFold(p, reqPath) {
			newPaths = append(newPaths, p)
		}
	}
	config.BasePaths = newPaths
	saveConfig(config)
	w.WriteHeader(http.StatusOK)
}

func handleAddPath(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")

	if reqPath != "" {
		exists := false
		for _, p := range config.BasePaths {
			if strings.EqualFold(p, reqPath) {
				exists = true
				break
			}
		}
		if !exists {
			config.BasePaths = append(config.BasePaths, reqPath)
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

func getRepoDetails(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	details := make(map[string]interface{})
	details["path"] = path

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
	details["summary"] = runGit("shortlog", "-sn", "--all")
	details["recent_activity"] = runGit("log", "-5", "--oneline")
	details["disk_usage"] = runGit("count-objects", "-vH")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}
