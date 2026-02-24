package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

func startWatcher(bases []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	repos := findRepos(bases)
	
	// Map to track changes with debounce
	pending := make(map[string]time.Time)
	var mu sync.Mutex

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Ignora alterações nos arquivos de ícones para evitar loop infinito
				name := strings.ToLower(event.Name)
				if strings.HasSuffix(name, "desktop.ini") || strings.HasSuffix(name, ".directory") {
					continue
				}

				repo := findRepoRoot(event.Name)
				if repo != "" {
					mu.Lock()
					pending[repo] = time.Now()
					mu.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	// Debounce goroutine
	go func() {
		for {
			time.Sleep(1 * time.Second)
			var toUpdate []string
			mu.Lock()
			now := time.Now()
			for repo, lastChange := range pending {
				if now.Sub(lastChange) >= 2*time.Second {
					toUpdate = append(toUpdate, repo)
					delete(pending, repo)
				}
			}
			mu.Unlock()

			if len(toUpdate) > 0 {
				var reallyUpdated []string
				for _, repo := range toUpdate {
					if updateRepo(repo, true) {
						reallyUpdated = append(reallyUpdated, repo)
					}
				}
				refreshUI(reallyUpdated)
			}
		}
	}()

	// Add repos to watcher recursively
	for _, repo := range repos {
		filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Pula pastas pesadas ou internas
				name := info.Name()
				if name == "node_modules" || name == "vendor" || name == "bin" || name == "obj" {
					return filepath.SkipDir
				}
				// Watch .git folder and its critical files
				if name == ".git" {
					watcher.Add(path)
					// Monitora arquivos vitais para mudanças de status/remoto/branch
					critical := []string{"config", "index", "HEAD", "FETCH_HEAD"}
					for _, c := range critical {
						cPath := filepath.Join(path, c)
						if _, err := os.Stat(cPath); err == nil {
							watcher.Add(cPath)
						}
					}
					return filepath.SkipDir
				}
				err = watcher.Add(path)
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

func findRepoRoot(path string) string {
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
