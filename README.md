# Mi Amor's Quantum Neural Hub

Welcome to Mi Amor's Quantum Neural Hub, a full-stack, production-grade AI application. It features a powerful Go backend and a stunning, animated, single-page HTML frontend, all packaged in a single Docker container for easy deployment to Hugging Face.

## Features

- **Production-ready Go backend** with AI integration
- **Stunning single-page HTML frontend** with animations and a futuristic "Mi Amor" theme
- **Unified Docker image** for easy deployment to Hugging Face
- **Local development environment** with Docker Compose

## Project Structure

```
.
├── backend/
│   ├── cmd/server/main.go
│   └── ... (internal services)
├── frontend/
│   └── index.html
├── Dockerfile
├── docker-compose.yml
├── nginx.conf
├── supervisord.conf
└── init.sql
```

## Deployment to Hugging Face

1.  **Create a Hugging Face Space**

    - Choose the "Docker" SDK and the "Blank" template.
    - Add your `KIMI_API_KEY`, `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `JWT_SECRET`, and `REDIS_URL` to the "Repository secrets".

2.  **Push your code**

    ```bash
    git remote add huggingface https://huggingface.co/spaces/YOUR_HF_USER/YOUR_SPACE_NAME
    git push huggingface main
    ```

    Hugging Face will automatically build and deploy your application.

## Local Development

For local development, you can use the provided `docker-compose.yml` file.

1.  **Create your Environment File**

    Copy the example environment file:
    ```bash
    cp .env.example .env
    ```

    Now, edit the `.env` file with your actual secret keys.

2.  **Launch the Application**

    Start all services with Docker Compose:
    ```bash
    docker compose up -d
    ```

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
