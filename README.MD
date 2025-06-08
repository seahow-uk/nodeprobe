# NodeProbe - Distributed Peer-to-Peer Network Monitor

NodeProbe is a distributed peer-to-peer system designed for network monitoring and topology discovery. Each node in the network operates independently while collaborating to maintain a comprehensive view of the entire network topology.

## 🏗️ Architecture

### Core Components

- **HTTPS Web Server** (Port 443): Provides secure API endpoints and web dashboard
- **Polling Service**: Periodically queries other nodes to measure connectivity and response times
- **Reporting Service**: Sends network snapshots to designated reporting servers
- **SQLite Database**: Stores node information and polling results locally
- **TLS Certificate Management**: Automatically generates self-signed certificates for HTTPS

### Key Features

- **Autonomous Operation**: Each node operates independently without central coordination
- **Dynamic Discovery**: Nodes discover each other through seed configurations and peer sharing
- **Network Resilience**: System continues operating even when nodes join or leave
- **Path MTU Discovery**: Automatically determines optimal packet sizes between nodes
- **Real-time Monitoring**: Continuous polling with configurable intervals
- **Web Dashboard**: Beautiful HTML interface for network visualization
- **Secure Communication**: All inter-node communication uses HTTPS with self-signed certificates

## 🚀 Quick Start

### Using Docker Compose (Recommended)

1. **Clone and build the system:**

   ```bash
   git clone <repository-url>
   cd nodeprobe
   docker-compose up --build
   ```

2. **Access the dashboards:**

   - Node 1: <https://localhost:8443/dashboard>
   - Node 2: <https://localhost:8444/dashboard>
   - Node 3: <https://localhost:8445/dashboard>
   - Node 4: <https://localhost:8446/dashboard>

3. **Monitor the logs:**

   ```bash
   docker-compose logs -f
   ```

### Manual Docker Build

```bash
# Build the image
docker build -t nodeprobe .

# Run a single node
docker run -d \
  --name nodeprobe-node \
  -p 8443:443 \
  -v nodeprobe_data:/app/data \
  -v nodeprobe_certs:/app/certs \
  nodeprobe
```

### Local Development

```bash
# Install dependencies
go mod download

# Run locally (requires Go 1.21+)
go run ./cmd/nodeprobe
```

## 📁 Project Structure

```
nodeprobe/
├── cmd/
│   └── nodeprobe/           # Main application entry point
│       └── main.go
├── internal/
│   ├── app/                 # Application services
│   │   ├── node_service.go
│   │   ├── polling_service.go
│   │   ├── reporting_service.go
│   │   └── web_server.go
│   ├── domain/              # Core business logic
│   │   ├── models.go
│   │   └── interfaces.go
│   └── pkg/                 # Infrastructure packages
│       ├── config/          # Configuration management
│       ├── http/            # HTTP client
│       ├── sqlite/          # Database repository
│       └── tls/             # TLS certificate management
├── configs/                 # Node configurations
│   ├── node1/
│   ├── node2/
│   ├── node3/
│   └── node4/
├── web/                     # Static web assets
├── scripts/                 # Utility scripts
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

## 🔧 Configuration

### Seed Configuration (`seed.json`)

```json
{
  "nodes": [
    {
      "fqdn": "node1.example.com",
      "ip": "192.168.1.100"
    },
    {
      "fqdn": "node2.example.com",
      "ip": "192.168.1.101"
    }
  ]
}
```

### Reporting Server Configuration (`reportingserver.json`)

```json
{
  "server_fqdn": "reporting.example.com",
  "server_ip": "192.168.1.10"
}
```

### Environment Variables

```bash
NODE_ENV=production
DATA_DIR=/app/data
CERT_DIR=/app/certs
```

## 🌐 API Endpoints

### Node Information

- **GET** `/nodeinfo` - Returns node details and known peers
- **GET** `/health` - Health check endpoint

### Network Reporting

- **POST** `/report` - Accepts network snapshots from other nodes

### Web Interface

- **GET** `/dashboard` - HTML dashboard for network visualization
- **GET** `/` - Redirects to dashboard

## 📊 Monitoring and Observability

### Dashboard Features

- **Network Topology**: Visual representation of all discovered nodes
- **Real-time Statistics**: Success rates, response times, node counts
- **Historical Data**: 24-hour polling history and trends
- **Node Status**: Active/inactive status with last seen timestamps
- **Path MTU Information**: Network path characteristics

### Health Checks

Each node provides health endpoints for monitoring:

```bash
curl -k https://localhost:8443/health
```

Response:

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "node_id": "uuid-here",
  "node_fqdn": "nodeprobe-1",
  "node_ip": "172.20.0.10",
  "known_nodes": 3
}
```

## 🔒 Security

### TLS/HTTPS

- All communication uses HTTPS with automatically generated self-signed certificates
- Certificates include all local network interfaces and hostnames
- Automatic certificate renewal when approaching expiration

### Network Security

- Self-signed certificates are accepted for peer-to-peer communication
- No external dependencies or internet access required
- Configurable timeouts and rate limiting

### Container Security

- Runs as non-root user inside containers
- Minimal attack surface with Alpine Linux base
- Read-only configuration mounts

## 🔄 Network Behavior

### Discovery Process

1. **Bootstrap**: Nodes start with seed configuration
2. **Peer Discovery**: Each node shares its known peers with others
3. **Continuous Polling**: Regular health checks maintain network view
4. **Dynamic Updates**: New nodes are automatically discovered and integrated

### Polling Strategy

- **Round-robin**: Nodes are polled in rotation
- **Configurable Interval**: Default 30-second polling interval
- **Timeout Handling**: Failed polls mark nodes as inactive
- **Path MTU Discovery**: Performed on first contact with each node

### Data Management

- **Local Storage**: Each node maintains its own SQLite database
- **Size Limits**: Automatic cleanup when database exceeds 10MB
- **Retention**: Configurable data retention policies

## 🛠️ Development

### Building from Source

```bash
# Clone repository
git clone <repository-url>
cd nodeprobe

# Install dependencies
go mod download

# Build binary
go build -o nodeprobe ./cmd/nodeprobe

# Run tests
go test ./...
```

### Adding New Features

1. **Domain Layer**: Add new models and interfaces in `internal/domain/`
2. **Application Layer**: Implement business logic in `internal/app/`
3. **Infrastructure**: Add supporting code in `internal/pkg/`
4. **Testing**: Include comprehensive tests for all new functionality

### Code Organization

The project follows Clean Architecture principles:

- **Domain**: Core business logic and interfaces
- **Application**: Use cases and application services
- **Infrastructure**: External concerns (database, HTTP, etc.)

## 🐛 Troubleshooting

### Common Issues

**Nodes not discovering each other:**

- Check seed.json configuration
- Verify network connectivity between containers
- Ensure HTTPS certificates are generated correctly

**High memory usage:**

- Check database size and cleanup frequency
- Monitor polling interval and number of nodes
- Review log retention settings

**Certificate errors:**

- Verify certificate generation in logs
- Check file permissions in cert directory
- Ensure proper hostname resolution

### Debugging

### _View logs_

```bash
docker-compose logs -f nodeprobe-1
```

### _Access container_

```bash
docker exec -it nodeprobe-1 sh
```

### _Check database_

```bash
sqlite3 /app/data/nodeprobe.db ".tables"
```

### _Testing Health Checks_

Each node exposes a health endpoint at https://`node-fqdn`:`port`/health

Test using: curl -k <https://localhost:8443/health> (for node 1)

The Docker Compose file already includes health checks that run every 30 seconds

### _Testing Node Information_

Check node details at https://`node-fqdn`:`port`/nodeinfo

Example: curl -k <https://localhost:8443/nodeinfo>

### _Testing Dashboard Access_

Access the web dashboard at https://`node-fqdn`:`port`/dashboard

Example: <https://localhost:8443/dashboard>

### _Testing Monitoring Points_

Watch the logs:

`docker logs nodeprobe-1 (and other nodes)`

Check data persistence in the mounted volumes

Monitor the SQLite database in

`/app/data/nodeprobe.db`

### _Testing System Load_

Test the system under load using tools like Apache Bench or JMeter

Focus on the main endpoints: `/health`, `/nodeinfo`, `/dashboard`

### _Testing Security Features_

Verify TLS certificates in `/app/certs`

## 📈 Performance Considerations

### Scalability

- **Node Limit**: Tested with up to 100 nodes
- **Polling Overhead**: O(n) where n is number of nodes
- **Database Size**: Automatic cleanup maintains performance
- **Memory Usage**: ~50MB per node under normal load

### Optimization

- **Polling Interval**: Adjust based on network size and requirements
- **Database Cleanup**: Configure retention based on storage constraints
- **Network Timeouts**: Tune for network latency characteristics
