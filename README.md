# DataCentral T&T

Welcome to DataCentral T&T, your business data hub with a Trini touch! This application provides a secure and easy way to upload, store, and manage your business documents with robust encryption.

## Features

-   **Secure File Upload**: Upload your important business documents (`.csv`, `.xlsx`, `.pdf`, `.txt`) with confidence.
-   **Robust Encryption**: All uploaded file metadata is encrypted using AES-256-GCM with a key derived using PBKDF2, ensuring your data is secure.
-   **Notion-Inspired UI**: A clean, modern, and mobile-first user interface inspired by Notion, tailored with the colors and spirit of Trinidad and Tobago.
-   **Local & Ready**: Designed with the needs of T&T businesses in mind, with familiar terminology and a welcoming feel.
-   **Scalable Storage**: Uses SQLite for reliable and scalable data storage, ready to grow with your business.

## Getting Started

### Prerequisites

-   Go 1.21 or later.

### Running Locally

1.  **Clone the repository**:
    ```sh
    git clone <repository-url>
    cd <repository-directory>
    ```

2.  **Install dependencies**:
    ```sh
    go mod tidy
    ```

3.  **Build the application**:
    ```sh
    go build -o datacentral-tt
    ```

4.  **Run the application**:
    ```sh
    ./datacentral-tt
    ```

The server will start on port `8080`. You can access the dashboard by opening `http://localhost:8080` in your web browser.

## Deployment

This application is designed for easy deployment to platforms like Railway.

1.  **Create a `railway.json` file**:
    ```json
    {
      "build": {
        "builder": "NIXPACKS"
      },
      "deploy": {
        "startCommand": "./datacentral-tt",
        "restartPolicyType": "ON_FAILURE"
      }
    }
    ```

2.  **Push to a GitHub repository and connect to Railway**. Railway will automatically build and deploy the application.

3.  **Set Environment Variables**: For production, set the `MASTER_KEY` environment variable to a strong, unique secret for encryption.

## API Endpoints

-   `GET /`: Serves the main HTML dashboard.
-   `POST /upload`: Handles file uploads, encrypts the metadata, and stores it.
-   `GET /query`: Retrieves and returns a list of all encrypted file records.
-   `GET /health`: A health check endpoint that returns the status of the server.
