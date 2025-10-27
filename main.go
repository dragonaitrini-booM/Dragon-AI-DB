package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	inputDir  string
	outputDir string
	workers   int
	scanFreq  time.Duration
)

func init() {
	flag.StringVar(&inputDir, "input", "./input", "input directory to watch for CSV files")
	flag.StringVar(&outputDir, "output", "./output", "output directory for reports")
	flag.IntVar(&workers, "workers", 6, "number of concurrent workers")
	flag.DurationVar(&scanFreq, "scan", 2*time.Second, "directory scan frequency")
}

func main() {
	flag.Parse()

	// Ensure directories exist
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		log.Fatalf("failed to create input dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("failed to create output dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan string, 100)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(ctx, id, jobs)
		}(i + 1)
	}

	// Start directory scanner
	scannerDone := make(chan struct{})
	go func() {
		defer close(scannerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(scanFreq):
				files, _ := filepath.Glob(filepath.Join(inputDir, "*.csv"))
				for _, f := range files {
					// Move file to temp path (atomic rename) to avoid re-processing
					abs, err := filepath.Abs(f)
					if err != nil {
						log.Printf("skipping %s: %v", f, err)
						continue
					}
					jobs <- abs
				}
			}
		}
	}()

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	fmt.Println("shutting down...")
	cancel()
	// wait for scanner to stop
	<-scannerDone
	// close jobs and wait for workers
	close(jobs)
	wg.Wait()

	fmt.Println("done")
}

func worker(ctx context.Context, id int, jobs <-chan string) {
	log.Printf("worker %d started", id)
	for {
		select {
		case <-ctx.Done():
			log.Printf("worker %d stopping (ctx)", id)
			return
		case jobPath, ok := <-jobs:
			if !ok {
				log.Printf("worker %d stopping (jobs closed)", id)
				return
			}
			log.Printf("worker %d processing %s", id, jobPath)
			if err := processAndReport(jobPath, outputDir); err != nil {
				log.Printf("worker %d failed to process %s: %v", id, jobPath, err)
				// move to .failed for inspection
				failedPath := jobPath + ".failed"
				os.Rename(jobPath, failedPath)
			} else {
				// remove original file after successful processing
				os.Remove(jobPath)
			}
		}
	}
}
