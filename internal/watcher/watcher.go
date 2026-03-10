package watcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/Kaiman42/reposync/internal/git"
	"github.com/Kaiman42/reposync/internal/ui"
)

func StartWatcher(bases []string) {
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("WATCHER PANIC: %v", r)
			fmt.Println(errStr)
			ui.ShowMessage("RepoSync - Erro no Watcher", errStr)
		}
	}()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	done := make(chan bool)
	repos := git.FindRepos(bases)

	pending := make(map[string]time.Time)
	var mu sync.Mutex

	go func() {
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}

				repo := FindRepoRoot(event.Name)
				if repo != "" {
					mu.Lock()
					pending[repo] = time.Now()
					mu.Unlock()
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			var toUpdate []string
			mu.Lock()
			now := time.Now()
			for repo, lastChange := range pending {
				if now.Sub(lastChange) >= 500*time.Millisecond {
					toUpdate = append(toUpdate, repo)
					delete(pending, repo)
				}
			}
			mu.Unlock()

			if len(toUpdate) > 0 {
				for _, repo := range toUpdate {
					// We need a way to update the repo. 
					// In the original it called updateRepo(repo, true)
					// updateRepo was in main.go
					// It just calls getGitStatus and updateDirectoryIcon
					status := git.GetGitStatus(repo)
					ui.UpdateDirectoryIcon(repo, status)
				}
			}
		}
	}()

	for _, repo := range repos {
		filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "node_modules" || name == "vendor" || name == "bin" || name == "obj" {
					return filepath.SkipDir
				}
				if name == ".git" {
					w.Add(path)
					critical := []string{"config", "index", "HEAD", "FETCH_HEAD"}
					for _, c := range critical {
						cPath := filepath.Join(path, c)
						if _, err := os.Stat(cPath); err == nil {
							w.Add(cPath)
						}
					}
					return filepath.SkipDir
				}
				err = w.Add(path)
				if err != nil {
					log.Printf("Warning: could not watch %s: %v", path, err)
				}
			}
			return nil
		})
	}

	log.Printf("Watching %d repositories...", len(repos))
	<-done
}

func FindRepoRoot(path string) string {
	curr := path
	for {
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}
