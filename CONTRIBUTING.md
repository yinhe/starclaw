# Contributing to StarClaw

Thank you for your interest in contributing to StarClaw! Every contribution matters — from bug reports to feature PRs.

## Quick Links

- [Issues](https://github.com/yinhe/starclaw/issues) — Bug reports & feature requests
- [Discussions](https://github.com/yinhe/starclaw/discussions) — Questions & ideas
- [Docs](docs/) — Technical documentation

## Getting Started

### Prerequisites

- **Go 1.24+**
- **Node.js 20+** (with npm)
- **Docker & Docker Compose** (for running dependencies)
- **MySQL 8.0** and **Redis 7** (or use Docker Compose)

### Development Setup

```bash
# 1. Fork and clone
git clone https://github.com/<your-username>/starclaw.git
cd starclaw

# 2. Configure environment
cp .env.example .env
# Edit .env: set JWT_SECRET and any API keys you need

# 3. Start infrastructure (MySQL + Redis)
docker compose up -d mysql redis

# 4. Run backend
cd api
go mod download
go run cmd/server/main.go

# 5. Run frontend (in another terminal)
cd web
npm install
npm run dev
```

The backend runs on `http://localhost:8080` and the frontend on `http://localhost:5173`.

## How to Contribute

### Reporting Bugs

1. Search [existing issues](https://github.com/yinhe/starclaw/issues) first
2. Include: steps to reproduce, expected vs actual behavior, environment info
3. Attach logs if available (check browser console and Docker logs)

### Suggesting Features

Open a [Discussion](https://github.com/yinhe/starclaw/discussions) first to gather feedback before writing code.

### Submitting Pull Requests

1. **Fork** the repository
2. **Create a branch** from `main`: `git checkout -b feat/my-feature`
3. **Make changes** following the coding guidelines below
4. **Test** your changes locally
5. **Commit** with clear messages (see below)
6. **Push** and open a PR against `main`

## Coding Guidelines

### Go (Backend)

- Follow standard Go conventions (`gofmt`, `golint`)
- Use `internal/` packages for business logic
- Add handler functions in `internal/api/v1/`
- Use GORM for database operations
- Error handling: wrap errors with context using `fmt.Errorf("context: %w", err)`

### TypeScript/React (Frontend)

- Use functional components with hooks
- Use TypeScript strictly (no `any` unless absolutely necessary)
- Style with TailwindCSS utility classes
- State management via Zustand stores
- Icons from `lucide-react`

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add workflow export to JSON
fix: prevent duplicate agent names
docs: update API reference for /v1/chat
chore: bump Go dependencies
```

### File Structure

```
api/internal/
├── api/v1/          # HTTP handlers (thin layer)
├── agent/           # Agent engine logic
├── provider/        # LLM provider adapters
├── rag/             # RAG pipeline
├── tool/            # Tool implementations
└── workflow/        # Workflow engine

web/src/
├── components/      # Reusable UI components
├── pages/           # Page-level components
├── stores/          # Zustand stores
├── services/        # API client functions
└── utils/           # Helpers
```

## Testing

```bash
# Backend tests
cd api && go test ./...

# Frontend type check
cd web && npx tsc --noEmit
```

## Code Review

All PRs require at least one review. Reviewers will check:

- **Correctness** — Does it work as intended?
- **Style** — Does it follow project conventions?
- **Tests** — Are edge cases covered?
- **Security** — No hardcoded secrets, proper input validation

## Community

- Be respectful and constructive
- Help others in issues and discussions
- Credit others' work appropriately

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

---

Thank you for helping make StarClaw better! 🦞
