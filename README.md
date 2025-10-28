# AI Database Application

This is a full-stack, production-grade AI database application. It features a powerful Go backend and a modern React frontend, designed for intuitive data analysis and visualization.

## Features

- **Production-ready Go backend** with AI integration
- **Modern React frontend** with real-time features
- **Secure authentication** with JWT tokens
- **AI-powered data analysis** with natural language processing
- **Advanced visualizations** with Chart.js optimization
- **Database integration** with query validation
- **Export capabilities** (PDF, Excel, CSV)
- **Docker deployment** ready
- **Comprehensive error handling** and logging
- **Mobile-responsive design**

## Project Structure

```
ai-database-app/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── ai/
│   │   ├── auth/
│   │   ├── database/
│   │   ├── handlers/
│   │   └── middleware/
│   ├── pkg/
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── hooks/
│   │   ├── services/
│   │   └── utils/
│   ├── package.json
│   └── vite.config.js
└── docker-compose.yml
```

## Quick Start

### 1. Backend Setup

```bash
cd backend
go mod tidy
go run cmd/server/main.go
```

### 2. Frontend Setup

```bash
cd frontend
npm install
npm run dev
```

### 3. Docker Setup

```bash
# Copy environment file
cp .env.example .env
# Edit .env with your actual API keys

# Start all services
docker-compose up -d
```

### 4. Database Migration (init.sql)

The `init.sql` file will be automatically run when you start the Docker container, creating the necessary tables and seeding the database with sample data.

## Environment Variables

Create a `.env` file in the root of the project and add the following variables:

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

# Frontend
VITE_API_URL=http://localhost:8080
```
