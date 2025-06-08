# Backend Directory Structure

This document outlines the complete backend directory structure for the K4-Chat application.

## Complete Directory Tree

```
backend/
├── cmd/                          # Application entry points
│   ├── api/                      # Main API server
│   │   └── main.go              # Server initialization & startup
│   └── worker/                   # Background job processor (future)
│       └── main.go
├── internal/                     # Private application code
│   ├── actor/                    # Actor implementations
│   │   ├── supervisor.go         # Root supervisor actor
│   │   ├── user_manager.go       # User lifecycle management
│   │   ├── user.go              # Individual user actor
│   │   ├── connection.go         # WebSocket connection handling
│   │   ├── chat_session.go       # Chat session management
│   │   ├── streaming.go          # LLM response streaming
│   │   ├── branching.go          # Conversation branching
│   │   ├── sync.go              # Cross-device synchronization
│   │   ├── llm_pool.go          # LLM provider pool management
│   │   ├── openai.go            # OpenAI provider actor
│   │   ├── anthropic.go         # Anthropic provider actor
│   │   ├── tools_manager.go     # Tools coordination
│   │   ├── web_search.go        # Web search actor
│   │   ├── image_gen.go         # Image generation actor
│   │   └── file_handler.go      # File attachment handling
│   ├── messages/                 # Actor message types
│   │   ├── types.go             # Core message definitions
│   │   ├── user.go              # User-related messages
│   │   ├── chat.go              # Chat-related messages
│   │   ├── llm.go               # LLM provider messages
│   │   └── tools.go             # Tool operation messages
│   ├── database/                 # Database layer
│   │   ├── postgres.go          # PostgreSQL connection & queries
│   │   ├── redis.go             # Redis connection & operations
│   │   ├── models/              # Database models
│   │   │   ├── user.go          # User model & queries
│   │   │   ├── chat_session.go  # Chat session model
│   │   │   ├── message.go       # Message model
│   │   │   ├── attachment.go    # Attachment model
│   │   │   └── shared_chat.go   # Shared chat model
│   │   └── migrations/          # Database migration files
│   │       ├── 001_initial.sql
│   │       ├── 002_chat_sessions.sql
│   │       ├── 003_messages.sql
│   │       ├── 004_attachments.sql
│   │       └── 005_shared_chats.sql
│   ├── auth/                     # Authentication & authorization
│   │   ├── jwt.go               # JWT token handling
│   │   ├── middleware.go        # Auth middleware
│   │   ├── password.go          # Password hashing/validation
│   │   └── session.go           # Session management
│   ├── websocket/               # WebSocket handling
│   │   ├── handler.go           # WebSocket upgrade & routing
│   │   ├── manager.go           # Connection management
│   │   ├── message.go           # Message parsing & validation
│   │   └── protocol.go          # WebSocket protocol definitions
│   ├── llm/                     # LLM provider abstractions
│   │   ├── interface.go         # Common LLM interface
│   │   ├── openai/              # OpenAI implementation
│   │   │   ├── client.go        # OpenAI API client
│   │   │   ├── streaming.go     # Streaming response handling
│   │   │   └── models.go        # Model definitions
│   │   ├── anthropic/           # Anthropic implementation
│   │   │   ├── client.go
│   │   │   ├── streaming.go
│   │   │   └── models.go
│   │   └── local/               # Local LLM implementation
│   │       ├── ollama.go        # Ollama integration
│   │       └── streaming.go
│   ├── tools/                   # External tool integrations
│   │   ├── interface.go         # Common tool interface
│   │   ├── search/              # Web search implementation
│   │   │   ├── searxng.go       # SearXNG integration
│   │   │   ├── brave.go         # Brave Search API
│   │   │   └── google.go        # Google Search API
│   │   ├── image/               # Image generation
│   │   │   ├── dalle.go         # DALL-E integration
│   │   │   ├── midjourney.go    # Midjourney API
│   │   │   └── stability.go     # Stability AI
│   │   └── files/               # File handling
│   │       ├── storage.go       # File storage interface
│   │       ├── s3.go           # S3 implementation
│   │       ├── local.go        # Local file storage
│   │       └── processor.go    # File processing utilities
│   ├── config/                  # Configuration management
│   │   ├── config.go           # Configuration structure
│   │   ├── validation.go       # Config validation
│   │   └── defaults.go         # Default values
│   ├── middleware/              # HTTP middleware
│   │   ├── cors.go             # CORS handling
│   │   ├── rate_limit.go       # Rate limiting
│   │   ├── logging.go          # Request logging
│   │   ├── recovery.go         # Panic recovery
│   │   └── security.go         # Security headers
│   ├── errors/                  # Error handling
│   │   ├── types.go            # Error type definitions
│   │   ├── handler.go          # Error handler
│   │   └── codes.go            # Error codes
│   └── utils/                   # Utility functions
│       ├── crypto.go           # Cryptographic utilities
│       ├── validation.go       # Input validation
│       ├── formatting.go       # String formatting
│       └── time.go             # Time utilities
├── pkg/                         # Public packages (importable)
│   ├── api/                     # API handlers & routes
│   │   ├── handlers/            # HTTP handlers
│   │   │   ├── auth.go          # Authentication endpoints
│   │   │   ├── user.go          # User management
│   │   │   ├── chat.go          # Chat operations
│   │   │   ├── message.go       # Message operations
│   │   │   ├── share.go         # Chat sharing
│   │   │   └── health.go        # Health check
│   │   ├── routes/              # Route definitions
│   │   │   ├── api.go           # API routes setup
│   │   │   ├── websocket.go     # WebSocket routes
│   │   │   └── public.go        # Public routes
│   │   └── middleware/          # API-specific middleware
│   │       └── context.go       # Request context handling
│   ├── types/                   # Shared type definitions
│   │   ├── user.go             # User types
│   │   ├── chat.go             # Chat types
│   │   ├── message.go          # Message types
│   │   ├── api.go              # API request/response types
│   │   └── websocket.go        # WebSocket message types
│   └── client/                  # Client libraries (future)
│       └── go/                  # Go client library
├── migrations/                  # Database migrations
│   ├── postgres/                # PostgreSQL migrations
│   │   ├── 001_initial.up.sql
│   │   ├── 001_initial.down.sql
│   │   ├── 002_chat_sessions.up.sql
│   │   ├── 002_chat_sessions.down.sql
│   │   └── ...
│   └── redis/                   # Redis setup scripts
│       └── setup.redis
├── scripts/                     # Development & deployment scripts
│   ├── dev/                     # Development scripts
│   │   ├── setup.sh            # Local development setup
│   │   ├── migrate.sh          # Database migration runner
│   │   └── seed.sh             # Database seeding
│   ├── build/                   # Build scripts
│   │   ├── docker.sh           # Docker build script
│   │   └── binary.sh           # Binary build script
│   └── deploy/                  # Deployment scripts
│       ├── staging.sh          # Staging deployment
│       └── production.sh       # Production deployment
├── configs/                     # Configuration files
│   ├── development.yaml        # Development config
│   ├── staging.yaml           # Staging config
│   ├── production.yaml        # Production config
│   └── docker.yaml            # Docker config
├── tests/                       # Test files
│   ├── integration/            # Integration tests
│   │   ├── actor_test.go       # Actor system tests
│   │   ├── websocket_test.go   # WebSocket tests
│   │   └── api_test.go         # API endpoint tests
│   ├── unit/                   # Unit tests
│   │   ├── auth_test.go        # Authentication tests
│   │   ├── database_test.go    # Database tests
│   │   └── llm_test.go         # LLM provider tests
│   └── fixtures/               # Test data
│       ├── users.json          # Test user data
│       └── messages.json       # Test message data
├── docs/                       # Documentation
│   ├── api/                    # API documentation
│   │   ├── openapi.yaml        # OpenAPI specification
│   │   └── websocket.md        # WebSocket protocol docs
│   ├── deployment/             # Deployment documentation
│   │   ├── docker.md           # Docker deployment
│   │   └── kubernetes.md       # Kubernetes deployment
│   └── development/            # Development documentation
│       ├── setup.md            # Development setup
│       └── architecture.md     # Architecture overview
├── deployments/                # Deployment configurations
│   ├── docker/                 # Docker configurations
│   │   ├── Dockerfile          # Main application Dockerfile
│   │   ├── Dockerfile.dev      # Development Dockerfile
│   │   └── docker-compose.yml  # Local development stack
│   ├── kubernetes/             # Kubernetes manifests
│   │   ├── namespace.yaml      # Namespace definition
│   │   ├── deployment.yaml     # Application deployment
│   │   ├── service.yaml        # Service definition
│   │   ├── ingress.yaml        # Ingress configuration
│   │   └── configmap.yaml      # Configuration map
│   └── terraform/              # Infrastructure as code
│       ├── main.tf             # Main Terraform config
│       ├── variables.tf        # Variable definitions
│       └── outputs.tf          # Output definitions
├── .env.example                # Environment variables example
├── .gitignore                  # Git ignore file
├── .dockerignore              # Docker ignore file
├── go.mod                     # Go module definition
├── go.sum                     # Go module checksums
├── Makefile                   # Build automation
└── README.md                  # Project documentation
```

## Key Directory Explanations

### `/cmd/`
Contains the main application entry points. Each subdirectory represents a separate binary.
- `api/`: Main HTTP/WebSocket server
- `worker/`: Background job processor (future expansion)

### `/internal/`
Private application code that cannot be imported by other projects.
- `actor/`: Core actor implementations using Hollywood
- `messages/`: Message type definitions for actor communication
- `database/`: Database layer with PostgreSQL and Redis
- `auth/`: Authentication and authorization logic
- `websocket/`: WebSocket handling and protocol implementation
- `llm/`: LLM provider abstractions and implementations
- `tools/`: External tool integrations (search, images, files)

### `/pkg/`
Public packages that can be imported by other projects.
- `api/`: HTTP handlers and routing
- `types/`: Shared type definitions
- `client/`: Client libraries for the API

### `/migrations/`
Database migration files organized by database type.

### `/scripts/`
Automation scripts for development, building, and deployment.

### `/configs/`
Configuration files for different environments.

### `/tests/`
Test files organized by test type (unit, integration).

### `/docs/`
Project documentation including API specs and deployment guides.

### `/deployments/`
Deployment configurations for different platforms (Docker, Kubernetes).

## File Naming Conventions

### Go Files
- Use snake_case for file names: `user_manager.go`
- Use descriptive names that match the primary type: `chat_session.go` for `ChatSessionActor`
- Group related functionality: `auth/jwt.go`, `auth/middleware.go`

### Test Files
- Follow Go convention: `filename_test.go`
- Mirror the structure of the code being tested

### Configuration Files
- Use environment names: `development.yaml`, `production.yaml`
- Use descriptive names for specific configs: `docker-compose.yml`

### Migration Files
- Use sequential numbering: `001_initial.sql`, `002_chat_sessions.sql`
- Include both up and down migrations: `.up.sql`, `.down.sql`
