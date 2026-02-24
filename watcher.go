package main

import (
	"log"
	"os"
	"path/filepath"
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
				for _, repo := range toUpdate {
					updateRepo(repo, true)
				}
				refreshUI(toUpdate)
			}
		}
	}()

	// Add repos to watcher
	for _, repo := range repos {
		err = watcher.Add(filepath.Join(repo, ".git"))
		if err != nil {
			// Some systems might not like watching the .git folder directly if it has many files
			// but for status changes it's the most reliable.
			log.Printf("Warning: could not watch %s: %v", repo, err)
		}
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
