# PodPerf

## Project Overview

PodPerf is a comprehensive observability playground that demonstrates the implementation of the three pillars of observability:

1. **Logs**: Capturing detailed application events using structured logging
2. **Metrics**: Measuring and monitoring application performance 
3. **Traces**: Tracking the flow of requests through distributed systems

The primary purpose of this project is to:
- Provide a hands-on learning environment for understanding observability concepts
- Demonstrate the intricacies involved in setting up a complete observability stack
- Serve as a playground for anyone wanting to experiment with modern observability tools
- Showcase integration patterns between different observability technologies

### Tech Stack

This project leverages a modern observability stack:

- **Application**: Go-based service with a merge sort algorithm
- **Logs**: 
  - Fluent Bit for log collection and forwarding
  - OpenSearch for log storage and search
  - OpenSearch Dashboards for log visualization
- **Metrics**:
  - Prometheus for metrics collection and storage
  - Grafana for metrics visualization and dashboarding
- **Traces**:
  - OpenTelemetry for instrumentation and trace collection
  - OpenTelemetry Collector for trace processing
  - Zipkin for distributed trace visualization

## Running with Docker Compose

This setup includes:
- Go application with merge sort algorithm
- Fluent Bit for log collection
- OpenSearch for log storage and visualization
- Prometheus for metrics collection
- Grafana for metrics visualization
- OpenTelemetry Collector for traces collection
- Zipkin for distributed tracing visualization

### Getting Started

1. Build and start the services:

```bash
docker-compose up -d
```

2. Check the logs:

```bash
docker-compose logs -f fluent-bit
```

3. Access the application:

```bash
curl http://localhost:8080/sort
```

### Load Testing

The project includes a load testing tool in the `loadtesting/` directory that uses the Vegeta library to generate load against the application.

1. Navigate to the loadtesting directory:

```bash
cd loadtesting
```

2. Run the load test:

```bash
go run .
```

This load testing tool is useful for:
- Generating consistent traffic to observe metrics in Prometheus/Grafana
- Testing system behavior under load

### Stopping and Cleaning Up

1. Stop all services but keep the data volumes:

```bash
docker-compose down
```

2. Stop all services and remove volumes (complete cleanup):

```bash
docker-compose down -v
```

3. Remove any remaining volumes and networks:

```bash
docker volume prune -f
docker network prune -f
```

4. Restart a specific service (e.g., the app):

```bash
docker-compose restart app
```

### Monitoring and Visualization

#### Logs

1. Monitor logs through Fluent Bit dashboard:
```
http://localhost:2020
```

2. Access OpenSearch Dashboards:
```
http://localhost:5601
```

#### Metrics

1. Access Prometheus:
```
http://localhost:9090
```

2. Access Grafana:
```
http://localhost:3000
```
Login with:
- Username: admin
- Password: admin

#### Traces

1. Access Zipkin UI:
```
http://localhost:9411
```

### Configuration

- App logs are shipped to Fluent Bit using the Fluentd log driver
- Fluent Bit parses JSON logs and forwards them to OpenSearch
- OpenSearch stores the logs in the "podperf-logs" index
- OpenSearch Dashboards provides log visualization
- App metrics are exposed at `/metrics` endpoint and collected by Prometheus
- Grafana connects to Prometheus for metrics visualization
- App traces are sent to OpenTelemetry Collector using OTLP over gRPC
- OpenTelemetry Collector processes and forwards traces to Zipkin
- Zipkin provides trace visualization and analysis

### Setting Up OpenSearch Dashboards

1. Open OpenSearch Dashboards at http://localhost:5601
2. Navigate to "Dashboards Management" from the left sidebar
3. Click on "Index Patterns" 
4. Click "Create index pattern"
5. Enter "podperf-logs*" as the pattern and click "Next step"
6. Select "time" as the time field from the dropdown
7. Click "Create index pattern" to finalize
8. Go to "Discover" in the left sidebar to view your logs

### Setting Up Grafana

1. Open Grafana at http://localhost:3000
2. Log in with admin/admin
3. Go to "Configuration" > "Data Sources"
4. Click "Add data source" and select "Prometheus"
5. Configure the Prometheus data source:
   - Name: Prometheus
   - URL: http://prometheus:9090
   - Access: Server (default)
   - Auth section:
     - Basic Auth: Disabled (no authentication is configured)
     - With Credentials: Disabled
     - TLS Client Auth: Disabled
     - Skip TLS Verify: Disabled
   - Prometheus details:
     - Scrape interval: 15s (matching the Prometheus config)
     - Query timeout: 60s
     - HTTP Method: GET
6. Click "Save & Test" to verify the connection
7. Create a new dashboard with panels for:
   - Total requests: `podperf_sort_requests_total`
   - Sort duration: `podperf_sort_duration_seconds_sum / podperf_sort_duration_seconds_count`
   - Error count: `podperf_errors_total`
   - Array size: `podperf_array_size`
   - Request rate: `rate(podperf_sort_requests_total[5m])`

### Using Zipkin for Distributed Tracing

1. Open Zipkin UI at http://localhost:9411
2. From the service dropdown, select "podperf-zipkin-service"
3. Click "Find Traces" to view the traces
4. Analyze the distributed traces of your sort operations:
   - View the complete request lifecycle
   - Examine the spans for each operation (sort handler, array generation, sorting)
   - Analyze performance bottlenecks
   - Correlate traces with logs and metrics for comprehensive observability