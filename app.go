package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Kaiman42/reposync/internal/config"
	"github.com/Kaiman42/reposync/internal/git"
	"github.com/Kaiman42/reposync/internal/ui"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetRepos() []RepoInfo {
	repos := git.FindRepos(cfg.BasePaths)
	var list []RepoInfo
	for _, p := range repos {
		status := git.GetGitStatus(p)
		modTime := git.GetRepoLastMod(p)
		list = append(list, RepoInfo{
			Path:         p,
			Name:         filepath.Base(p),
			Status:       status,
			Size:         getFolderSize(p),
			LastChange:   modTime.Format("02/01/2006 15:04"),
			RelativeTime: formatRelativeTime(modTime),
			RemoteURL:    git.GetRemoteURL(p),
		})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

type RepoInfo struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Size         string `json:"size"`
	LastChange   string `json:"last_change"`
	RelativeTime string `json:"relative_time"`
	RemoteURL    string `json:"remote_url"`
}

func (a *App) GetConfig() []string {
	return cfg.BasePaths
}

func (a *App) AddPath(path string) {
	if path != "" {
		exists := false
		for _, p := range cfg.BasePaths {
			if strings.EqualFold(p, path) {
				exists = true
				break
			}
		}
		if !exists {
			cfg.BasePaths = append(cfg.BasePaths, path)
			config.SaveConfig(cfg)
		}
	}
}

func (a *App) RemovePath(path string) {
	newPaths := []string{}
	for _, p := range cfg.BasePaths {
		if !strings.EqualFold(p, path) {
			newPaths = append(newPaths, p)
		}
	}
	cfg.BasePaths = newPaths
	config.SaveConfig(cfg)
}

func (a *App) OpenAction(path, action string) {
	switch action {
	case "explorer":
		if runtime.GOOS == "windows" {
			exec.Command("explorer", path).Run()
		} else {
			exec.Command("xdg-open", path).Run()
		}
	case "code":
		cmd := exec.Command("code", path)
		cmd.SysProcAttr = ui.GetSysProcAttr()
		cmd.Run()
	case "remote":
		if path != "" {
			openBrowser(path)
		}
	}
}

func (a *App) GetRepoDetails(path string) map[string]interface{} {
	details := make(map[string]interface{})
	details["path"] = path

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		cmd.SysProcAttr = ui.GetSysProcAttr()
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}

	details["branch"] = runGit("rev-parse", "--abbrev-ref", "HEAD")
	details["last_commit"] = runGit("log", "-1", "--format=%B")
	details["last_author"] = runGit("log", "-1", "--format=%an <%ae>")
	details["last_date"] = runGit("log", "-1", "--format=%ai")
	details["commit_count"] = runGit("rev-list", "--count", "HEAD")
	details["remote_url"] = git.GetRemoteURL(path)
	details["tags"] = runGit("tag")
	details["stashes"] = runGit("stash", "list")
	details["summary"] = runGit("shortlog", "-sn", "--all")
	details["recent_activity"] = runGit("log", "-5", "--oneline")
	details["disk_usage"] = runGit("count-objects", "-vH")

	return details
}
