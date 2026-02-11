# Quick Start Guide - Go Performance Enablement

## Prerequisites

```bash
# Install Go 1.24.3+
brew install go

# Install Docker Desktop
brew install --cask docker

# Install AWS CLI
brew install awscli

# Install kubectl
brew install kubectl

# Verify installations
go version
docker --version
aws --version
kubectl version --client
```

## Local Development Setup (One Command)

```bash
# Clone and setup
cd /Users/timothy.andrus/dev/go-performance-enablement

# Run automated setup
make setup-local
```

This will:
- ✅ Start Kafka, Zookeeper, Schema Registry
- ✅ Start LocalStack (DynamoDB, SQS, EventBridge, Secrets Manager)
- ✅ Start Prometheus and Grafana
- ✅ Create Kafka topics
- ✅ Register Avro schemas
- ✅ Create AWS resources
- ✅ Build and start Go applications

## Access Services

After setup completes:

- **Kafka UI**: http://localhost:8080
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Kafka Consumer Metrics**: http://localhost:9091/metrics
- **LocalStack**: http://localhost:4566

## Common Commands

```bash
# View all available commands
make help

# Start services
make start

# Stop services
make stop

# View logs
make logs-consumer

# Run tests
make test

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Build Lambda functions
make build-lambdas

# Deploy to dev
make deploy-dev
```

## Project Status

✅ **Complete**:
- 5 Lambda functions (Go)
- Kafka consumer for EKS
- Shared packages (events, awsutils, metrics)
- Docker & Kubernetes manifests
- Local development environment
- Makefile with 30+ commands
- Complete documentation

📋 **Remaining** (optional):
- ARCHITECTURE_KAFKA.md - Kafka/Qlik CDC architecture details
- BENCHMARKS.md - Performance comparison data
- GitHub Actions workflows - CI/CD automation
- Terraform modules - Infrastructure as Code

## Next Steps

1. **Explore the code**:
   ```bash
   code .
   ```

2. **Run locally**:
   ```bash
   # Start everything
   make start
   
   # Watch consumer logs
   make logs-consumer
   ```

3. **Test Kafka**:
   ```bash
   # List topics
   make kafka-topics
   
   # Consume from topic
   make kafka-consume TOPIC=qlik.customers
   ```

4. **Build for deployment**:
   ```bash
   # Build Lambda functions
   make build-lambdas
   
   # Build Docker image
   make docker-build
   ```

5. **Deploy**:
   ```bash
   # Deploy to dev
   make deploy-dev
   
   # Deploy to prod (with confirmation)
   make deploy-prod
   ```

## Troubleshooting

### Kafka not starting
```bash
# Check Docker resources (needs 4GB+ RAM)
docker stats

# Restart services
make stop
make start
```

### LocalStack not responding
```bash
# Check health
curl http://localhost:4566/_localstack/health

# Restart
docker-compose restart localstack
```

### Go build errors
```bash
# Update dependencies
make deps

# Clean and rebuild
make clean
make build
```

## Documentation

- `README.md` - Project overview and features
- `ARCHITECTURE.md` - System architecture (complete)
- `FINAL_SUMMARY.md` - Complete project summary
- `Makefile` - All available commands

## Support

For issues or questions:
1. Check `make help` for available commands
2. View logs: `make logs`
3. Consult FINAL_SUMMARY.md for project details

---

**Ready to code!** 🚀
