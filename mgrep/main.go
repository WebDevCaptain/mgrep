package main

import (
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/webdevcaptain/mgrep/worker"
	"github.com/webdevcaptain/mgrep/worklist"
	"os"
	"path/filepath"
	"sync"
)

func discoverDirs(wl *worklist.Worklist, path string) {
	entries, err := os.ReadDir(path)

	if err != nil {
		fmt.Println("ReadDir error:", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			nextPath := filepath.Join(path, entry.Name())
			discoverDirs(wl, nextPath)
		} else {
			wl.Add(worklist.NewJob(filepath.Join(path, entry.Name())))
		}
	}
}

var args struct {
	SearchTerm string `arg:"positional,required"`
	SearchDir  string `arg:"positional"`
}

func main() {
	fmt.Println("mgrep tool.")

	arg.MustParse(&args)

	var workersWg sync.WaitGroup

	wl := worklist.New(100)
	results := make(chan worker.Result, 100)

	numWorkers := 10
	workersWg.Add(1)

	go func() {
		defer workersWg.Done()

		discoverDirs(&wl, args.SearchDir)
		wl.Finalize(numWorkers)
	}()

	for i := 0; i < numWorkers; i++ {
		workersWg.Add(1)

		go func() {
			defer workersWg.Done()

			for {
				workEntry := wl.Next()

				if workEntry.Path != "" {
					workerResult := worker.FindInFile(workEntry.Path, args.SearchTerm)

					if workerResult != nil {
						for _, r := range workerResult.Inner {
							results <- r
						}
					}
				} else {
					return
				}
			}
		}()
	}

	blockWorkersWg := make(chan struct{})

	go func() {
		workersWg.Wait()
		close(blockWorkersWg)
		// fmt.Println("closed blockWorkersWg")
	}()

	var displayWg sync.WaitGroup

	displayWg.Add(1)
	go func() {
		for {
			select {
			case r := <-results:
				fmt.Printf("%v[%v]:%v\n", r.Path, r.LineNum, r.Line)

			case _, ok := <-blockWorkersWg:
				// fmt.Println("blockWorkersWg closed", len(results))

				if !ok {
					// fmt.Println("blockWorkersWg closed", len(results))

					if len(results) == 0 {
						displayWg.Done()
						return
					}
				}

			}
		}
	}()

	displayWg.Wait()
}
