Trini Data Pipeline

A small concurrent CSV processing pipeline (Go).

What it does
- Watches ./input for CSV files (polling every 2s by default)
- Processes files concurrently with a worker pool
- Produces simple JSON reports in ./output and deletes the original CSV on success

Build

Make sure Go 1.21+ is installed and available on PATH. From repository root:

    go build -o trini-pipeline

Run

Create directories and run with defaults:

    ./trini-pipeline

Optional flags:

    -input ./input -output ./output -workers 6 -scan 2s

Try it

1. Start the pipeline: ./trini-pipeline
2. Drop a CSV into ./input, e.g.:

   filename,amount,region
   a.csv,100,NA
   b.csv,200,EU

3. After processing a file you'll see a JSON report placed into ./output named <original>.report.json

Notes
- This is a simple, dependency-free demo focusing on concurrency and pipelines. It intentionally uses only standard library packages to avoid module checksum issues.
- For production: replace polling with fsnotify, add retries, observability, and move to proper durable storage.
