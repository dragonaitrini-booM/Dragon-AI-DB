# Mi Amor's Quantum Neural Hub

Welcome to Mi Amor's Quantum Neural Hub, a full-stack, production-grade AI application. It features a powerful Go backend and a stunning, animated, single-page HTML frontend.

## Features

- **Production-ready Go backend** with AI integration
- **Stunning single-page HTML frontend** with animations and a futuristic "Mi Amor" theme
- **Docker deployment** ready for a simple and fast launch

## Project Structure

```
.
├── backend/
│   ├── cmd/server/main.go
│   └── ... (internal services)
├── frontend/
│   └── index.html
├── docker-compose.yml
└── init.sql
```

## Quick Start with Docker

The entire application is designed to be run with a single command.

1.  **Create your Environment File**

    Copy the example environment file:
    ```bash
    cp .env.example .env
    ```

    Now, edit the `.env` file with your actual secret keys for `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `KIMI_API_KEY`, and `JWT_SECRET`.

2.  **Launch the Application**

    Start all services with Docker Compose:
    ```bash
    docker-compose up -d
    ```

    This command will:
    - Build and start the Go backend.
    - Start the PostgreSQL database and run the `init.sql` script.
    - Start the Redis cache.
    - Start an `nginx` web server to serve your beautiful `index.html`.

3.  **Access Your Masterpiece**

    - **Frontend:** [http://localhost:3000](http://localhost:3000)
    - **Backend Health Check:** [http://localhost:8080/health](http://localhost:8080/health)

## Environment Variables

Your `.env` file should contain the following variables:

```bash
# Database
DATABASE_URL=postgres://postgres:password@localhost:5432/aidatabase?sslmode=disable

# Supabase
SUPABASE_URL=your_supabase_project_url
SUPABASE_ANON_KEY=your_supabase_anon_key

# AI Service
KIMI_API_KEY=your_kimi_api_key

# Authentication
JWT_SECRET=your_jwt_secret_key_here

# Redis
REDIS_URL=redis://localhost:6379

# Server
PORT=8080
ENVIRONMENT=development
```
