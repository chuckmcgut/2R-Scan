# 2R-Scan

Lorcana card scanner with AI-powered PSA grade estimation.

## Quick Start

### Prerequisites

- Go 1.22+
- `go mod` available

### Running the Server

```bash
# Start the server
go run ./cmd/server

# The server will start on http://localhost:8080
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check - returns `{"status": "ok"}` |
| GET | `/api/v1/cards/search?name={name}` | Search cards by name |
| POST | `/api/v1/scan` | Upload image and get AI grade estimation |

**Scan Response:**
```json
{
  "name": "Mickey Mouse",
  "grade": 8.5,
  "c": 10,
  "co": 10,
  "e": 10,
  "s": 10,
  "image_url": "data:image/png;base64,...",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Running Tests

**Unit Tests:**
```bash
# Run all unit tests
go test ./... -v

# Run specific package tests
go test ./cmd/server -v
```

**Integration Tests:**
```bash
# Run the test suite script
./scripts/run-test.sh

# Or run integration tests directly
go test ./tests/... -v
```

### Importing Cards

Import card data from a JSON file:

```bash
# Create a cards.json file
cat > cards.json << EOF
[
  {
    "name": "Mickey Mouse",
    "c": 12,
    "co": 10,
    "e": 10,
    "s": 10,
    "grade": 8.5,
    "image_url": "https://example.com/mickey.jpg"
  }
]
EOF

# Import cards
go run ./cmd/server import cards.json
```

### Command Line Tools

**Test Image Decoding:**
```bash
# Test if an image can be decoded
go run ./cmd/server test image.jpg
```

**Import Cards:**
```bash
# Import card data from JSON file
go run ./cmd/server import cards.json
```

## Deployment

### Docker Deployment

Build the Docker image:

```bash
docker build -t 2r-scan .
```

Run the container:

```bash
docker run -p 8080:8080 -v $(pwd)/cards.db:/app/cards.db 2r-scan
```

The `-v` flag mounts the `cards.db` database file so the server persists data between containers.

Environment Variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port to listen on |
| `DATABASE_URL` | `cards.db` | Path to the database file |
| `SCAN_DIR` | `.` | Directory to scan for new images |

### Running with Docker Compose

Create a `docker-compose.yml`:

```yaml
version: '3.8'
services:
  2r-scan:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./cards.db:/app/cards.db
    environment:
      - PORT=8080
      - DATABASE_URL=/app/cards.db
      - SCAN_DIR=/app
```

Then run:

```bash
docker-compose up
```

### Building the Docker Image

The Dockerfile builds a multi-stage image:

1. **Builder stage**: Downloads Go dependencies and compiles the server binary
2. **Runtime stage**: Uses Alpine Linux with only the necessary runtime dependencies

The resulting image is small (~15MB) and optimized for production use.

## Architecture

## Architecture

```
2R-Scan/
├── cmd/server/          # HTTP server
│   └── main.go
├── internal/
│   ├── api/             # API definitions
│   └── scanner/         # Image processing
├── tests/                # Integration tests
└── scripts/
    └── run-test.sh      # Test runner script
```

## Development

### Adding New Cards

Use the Python import script:

```bash
python3 scripts/import_cards.py --file cards.json
```

### API Documentation

See [API.md](API.md) for detailed API documentation.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License
