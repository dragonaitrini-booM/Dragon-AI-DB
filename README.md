# Dragon-AI-DB

This repository contains two main components: a concurrent CSV processing pipeline and a minimal encrypted web server, both written in Go.

## Concurrent CSV Processing Pipeline

A small, efficient, and concurrent CSV processing pipeline.

### What it does

- Watches an `input` directory for new CSV files (polling every 2 seconds by default).
- Processes these files concurrently using a worker pool.
- For each processed CSV, it generates a simple JSON report in an `output` directory.
- Deletes the original CSV file upon successful processing.

### Getting Started

#### Prerequisites

- Go 1.21 or later installed and available on your `PATH`.

#### Building

From the repository root, run the following command to build the pipeline executable:

```sh
go build -o trini-pipeline
```

#### Running

1.  Create the input and output directories:
    ```sh
    mkdir -p input output
    ```
2.  Run the pipeline with default settings:
    ```sh
    ./trini-pipeline
    ```

#### Command-line Flags

You can customize the pipeline's behavior using the following optional flags:

-   `-input`: The directory to watch for CSV files. (Default: `./input`)
-   `-output`: The directory where JSON reports will be written. (Default: `./output`)
-   `-workers`: The number of concurrent worker goroutines. (Default: `6`)
-   `-scan`: The frequency at which to scan the input directory. (Default: `2s`)

Example:
```sh
./trini-pipeline -input /path/to/csvs -output /path/to/reports -workers 10 -scan 5s
```

### Try it Out

1.  Start the pipeline: `./trini-pipeline`
2.  Create a CSV file (e.g., `data.csv`) in the `./input` directory with some content:

    ```csv
    filename,amount,region
    a.csv,100,NA
    b.csv,200,EU
    c.csv,300,APAC
    ```
3.  After a few seconds, you will see a corresponding JSON report (`data.csv.report.json`) in the `./output` directory.

## Minimal Encrypted Server

The repository also includes a minimal, standalone web server implemented in `encrypted_server.go`.

### Purpose

This server is a demonstration of basic in-memory data encryption and decryption. It exposes a few simple API endpoints to add and query encrypted records, which are persisted to a local JSON file (`encrypted_records.json`).

**Note:** This server is for demonstration purposes only and is **not** suitable for production use due to its simplified security practices (e.g., simple key derivation, local key storage).

### Features

-   **Encryption**: Uses AES-256-GCM for authenticated encryption.
-   **Storage**: Persists encrypted records to a local JSON file.
-   **API**: Provides simple endpoints for health checks, adding sample data, and querying decrypted records.

### Running the Server

You can run the server directly using `go run`:

```sh
go run encrypted_server.go
```

The server will start on port `9090`. You can set the `MASTER_KEY` environment variable to provide a custom encryption key.

### API Endpoints

-   `GET /health`: Returns the server's status and encryption configuration.
-   `POST /add_sample`: Creates a sample record, encrypts it, and saves it to the database file.
-   `GET /query`: Loads all records, decrypts them, and returns the plaintext data.

## Project Structure

-   `main.go`: The main entry point for the CSV processing pipeline.
-   `processor.go`: Contains the core logic for reading, processing, and summarizing CSV files.
-   `config.go`: Handles loading configuration from environment variables and `.env` files.
-   `encrypted_server.go`: A standalone, minimal web server for demonstrating encryption.
-   `go.mod`: Go module definition.
-   `Dockerfile`: A sample Dockerfile for containerizing the pipeline.

## Contributing

Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for details on how to contribute to this project.
