# 🛡️ nox — An Intrusion Detection System & Threat Hunting Engine

`nox` is a stateful Intrusion Detection System (IDS) engine written in Go from scratch. The purpose of this project is to explore and implement the core mechanics behind modern detection and response platforms, with a special focus on event correlation and attack chain analysis.

This project demonstrates how to build a security engine that can ingest log data in real-time, analyze it against multiple layers of detection logic, and provide a powerful gRPC API for security analysts to proactively hunt for threats in historical data.

### Key Technologies

- Go: For the core detection engine, API server, and clients.
- gRPC / Protobuf: For high-performance, strongly-typed client-server communication.
- Elasticsearch: As a scalable, durable backend for storing and querying all event data.
- Docker / Docker Compose: For containerizing the entire application stack for easy deployment.
- Prometheus: For exporting critical application and detection metrics.
- Cobra & Viper: For building a professional, user-friendly CLI experience.

## Architecture
The `nox` ecosystem consists of several independent components that communicate over the network, simulating a real-world distributed security system.

```
                    +--------------------------------+
                    |        Threat Analyst          |
                    +--------------------------------+
                                   |
                                (gRPC)
                                   |
  +----------------------+         v          +-------------------------+
  | Log Simulator        |       +-----------+   (TCP)   +-------------------------+
  | (cmd/log-simulator)  |-----> | Log File  |---------> |      nox Engine         |
  +----------------------+       +-----------+           |  (cmd/nox, internal/)   |
                                                         |                         |
                                                         | - Ingester              |
                                                         | - Rule Engine           |
                                                         | - gRPC Server           |
                                                         +-------------------------+
                                                                |         ^
                                                                | (HTTP)  | (gRPC)
                                                                v         |
                                                         +-------------------------+
                                                         |  Elasticsearch & Kibana |
                                                         +-------------------------+


```

1. **Log Generation:** The log-simulator writes attack scenarios to a log file.
2. **Ingestion:** The nox engine tails the log file, parses the lines, and enriches the data.
3. **Detection & Alerting:** The Rule Engine analyzes the event stream, firing alerts for suspicious activity.
4. **Data Persistence:** All processed events are indexed into Elasticsearch.
5. **Threat Hunting:** An analyst uses the nox-cli to send gRPC requests to the nox engine, which then queries Elasticsearch to find historical data.

## Features

- **Stateless Detection Engine:** Utilizes a flexible, YAML-based rule engine for high-speed pattern matching. Rules are mapped to the MITRE ATT&CK® Framework.
- **Stateful Anomaly Detection:** Employs Go-based rules to track state over time and detect anomalies that span multiple events, such as SSH brute-force attacks.
- **Event Correlation Engine:** Connects seemingly disparate events to uncover multi-stage attack chains like "Download & Execute" or "Brute-Force & Evasion."
- **gRPC Threat Hunting API:** A high-performance, strongly-typed API that allows an analyst to query historical event data. Key methods include:
  - SearchEvents: For flexible, filter-based searches.
  - GetTopEvents: For statistical analysis and finding the "most common" events.
  - GetProcessAncestry: For walking the process tree to find the root cause of an event.
- **CLI Client:** nox-cli provides a polished, user-friendly interface for interacting with the gRPC API, complete with subcommands, flags, and formatted table output.

## How To Run

The entire environment is containerized and can be launched with **Docker Compose**.

1. **Start the System:** From the root of the nox project, run:

```
docker-compose up --build
```

2. **Generate Test Events**
   In a separate terminal, use the refactored log-simulator to generate test data. \
   First build the binary:

```
go build -o log-simulator ./cmd/log-simulator
```

Then, run a test:

- To run a specific attack chain:

```
# Test the Brute-Force & Defense Evasion correlation
./log-simulator --scenario=bruteforce

# Test the Download & Execute correlation
./log-simulator --scenario=download
```

- To run a continuous stream of random events:

```
./log-simulator --continuous
```

Observe the logs in your docker-compose terminal to see alerts being generated in real-time.

3. **Threat Hunt with `nox cli`**

In a third terminal, use the `nox-cli` to query the data you just generated. \
First, build the binary:

```
go build -o nox-cli ./cmd/nox-cli
```

Then, run some queries:

- Find the top 5 most common process names:

```
./nox-cli top process_name --n 5
```

- Search for a specific command:

```
# Find the defense evasion command from the 'bruteforce' scenario
./nox-cli search --filter command="history -c"
```

- Find the process ancestry for a given PID: (Use a PID from the search command above)

```
./nox-cli ancestry <PID_FROM_SEARCH>
```

4. **View Observability & Data**
- Prometheus Metrics: `http://localhost:9090/metrics`
- Kibana UI: `http://localhost:5601` (You can explore the raw event data in the process_executed and other indices).

## Project Structure
The project follows the standard Go project layout to ensure a clean separation of concerns.
- `cmd/` Contains the entrypoints for the runnable applications (nox, nox-cli, log-simulator).
- `internal/` Contains all the core library code for the project, which is not meant to be imported by other projects.
- `proto/` Contains the protobuf definition for the gRPC API contract.

## Example Alert (Correlation)

Here is an example of a **CRITICAL** alert generated by the correlation engine, showing a confirmed attack chain.

```
{
    "time": "2025-09-08T11:45:00.123Z",
    "level": "ERROR",
    "msg": "Attack Chain Detected: A successful login from 198.51.100.99 after a brute-force was followed by the defense evasion command: 'history -c'",
    "rule_name": "CorrelatedBruteForceAndEvasion",
    "severity": "CRITICAL",
    "source": "198.51.100.99",
    "metadata": {
        "mitre_tactic": "TA0005",
        "correlated_events": "TooManyFailedLogins, SSHD_Accepted_Password, DefenseEvasionCommand",
        "source_ip": "198.51.100.99",
        "evasion_command": "history -c",
        "linked_sshd_pid": "2538"
    }
}
```
