# MiniRedis

> A lightweight Redis-compatible in-memory database built from scratch in Go.

MiniRedis is a from-scratch implementation of a Redis-style key-value database server, designed to explore the internals behind networked databases.

It implements a TCP server, RESP protocol parsing, concurrent client handling, in-memory data structures, Redis-style commands, and Append Only File (AOF) persistence.

The project focuses on understanding the systems underneath a database rather than treating the database as a black box.

---

## Overview

MiniRedis follows the same fundamental architecture used by networked in-memory databases:

```mermaid
flowchart LR
    C[Redis Client] -->|TCP / RESP| S[MiniRedis Server]

    S --> P[RESP Parser]
    P --> H[Command Handler]

    H --> D[(In-Memory Data Store)]
    H --> A[AOF Persistence]

    A --> F[(appendonly.aof)]

    F -.->|Replay on startup| D

    classDef client fill:#161b22,stroke:#58a6ff,color:#fff
    classDef server fill:#0d1117,stroke:#7ee787,color:#fff
    classDef component fill:#161b22,stroke:#bc8cff,color:#fff
    classDef storage fill:#161b22,stroke:#ffa657,color:#fff

    class C client
    class S server
    class P,H component
    class D,A,F storage
```

A client communicates with the server over TCP using the RESP protocol. Requests are parsed, dispatched to the appropriate command handler, executed against the in-memory store, and returned to the client as RESP responses.

Write operations are additionally recorded in the AOF so that the database state can be reconstructed after a restart.

---

## Key Features

* TCP database server
* RESP protocol parser and serializer
* Concurrent client handling using Go goroutines
* In-memory key-value storage
* String data type
* Hash data type
* Redis-style command execution
* Append Only File persistence
* Database recovery through AOF replay
* Modular and minimal codebase
* Compatible with Redis-style clients for supported commands

---

## Architecture

The server is intentionally divided into small components with clear responsibilities.

```mermaid
flowchart TB
    subgraph Network["Network Layer"]
        TCP["TCP Listener"]
        CONN["Client Connection"]
    end

    subgraph Protocol["Protocol Layer"]
        READER["RESP Reader"]
        WRITER["RESP Writer"]
    end

    subgraph Execution["Execution Layer"]
        HANDLER["Command Handler"]
        COMMANDS["Command Dispatcher"]
    end

    subgraph Storage["Storage Layer"]
        STORE[("In-Memory Store")]
        HASH["Hash Operations"]
    end

    subgraph Persistence["Persistence Layer"]
        AOF["AOF Manager"]
        FILE[("appendonly.aof")]
    end

    TCP --> CONN
    CONN --> READER
    READER --> HANDLER
    HANDLER --> COMMANDS

    COMMANDS --> STORE
    COMMANDS --> HASH

    COMMANDS --> AOF
    AOF --> FILE

    STORE --> WRITER
    HASH --> WRITER
    WRITER --> CONN

    FILE -. "Replay" .-> STORE

    classDef network fill:#0d1117,stroke:#58a6ff,color:#fff
    classDef protocol fill:#0d1117,stroke:#bc8cff,color:#fff
    classDef execution fill:#0d1117,stroke:#7ee787,color:#fff
    classDef storage fill:#0d1117,stroke:#ffa657,color:#fff
    classDef persistence fill:#0d1117,stroke:#f778ba,color:#fff

    class TCP,CONN network
    class READER,WRITER protocol
    class HANDLER,COMMANDS execution
    class STORE,HASH storage
    class AOF,FILE persistence
```

### Component Responsibilities

| Component          | Responsibility                           |
| ------------------ | ---------------------------------------- |
| TCP Listener       | Accepts incoming client connections      |
| RESP Reader        | Parses client requests                   |
| Command Handler    | Processes individual client requests     |
| Command Dispatcher | Routes commands to their implementations |
| In-Memory Store    | Maintains active database state          |
| RESP Writer        | Encodes responses sent to clients        |
| AOF Manager        | Persists write operations                |
| AOF File           | Stores commands required for recovery    |

---

## Request Lifecycle

Every client request follows a simple execution pipeline:

```mermaid
sequenceDiagram
    autonumber

    participant C as Client
    participant S as MiniRedis
    participant R as RESP Reader
    participant H as Command Handler
    participant D as Data Store
    participant A as AOF

    C->>S: TCP connection
    C->>R: RESP command
    R->>H: Parsed command
    H->>D: Execute operation

    alt Write operation
        H->>A: Append command
        A-->>H: Persisted
    end

    D-->>H: Result
    H->>C: RESP response
```

For example:

```text
SET name Sahil
```

becomes:

```mermaid
flowchart LR
    A["SET name Sahil"] --> B["RESP Decode"]
    B --> C["Command Dispatch"]
    C --> D["Update Memory"]
    C --> E["Append to AOF"]
    D --> F["RESP Encode"]
    E --> F
    F --> G["Client"]
```

---

## Persistence

MiniRedis uses an **Append Only File (AOF)** approach for basic durability.

Instead of periodically writing the complete database state to disk, write operations are appended to a log.

```mermaid
flowchart LR
    CMD["Write Command"] --> MEM[("Memory")]
    CMD --> AOF["AOF Manager"]
    AOF --> LOG[("appendonly.aof")]

    LOG -->|Server Restart| REPLAY["Replay Commands"]
    REPLAY --> MEM

    classDef command fill:#161b22,stroke:#58a6ff,color:#fff
    classDef memory fill:#161b22,stroke:#7ee787,color:#fff
    classDef persistence fill:#161b22,stroke:#ffa657,color:#fff

    class CMD command
    class MEM memory
    class AOF,LOG,REPLAY persistence
```

### Recovery

When the server starts:

1. Open the AOF file.
2. Read previously persisted commands.
3. Parse each command.
4. Replay the commands against the in-memory store.
5. Restore the database state.

This provides persistence while keeping the runtime data path entirely in memory.

---

## Concurrency Model

Each client connection is handled independently using a Go goroutine.

```mermaid
flowchart TB
    S["MiniRedis Server"]

    S --> C1["Client 1"]
    S --> C2["Client 2"]
    S --> C3["Client 3"]
    S --> CN["Client N"]

    C1 --> G1["Goroutine"]
    C2 --> G2["Goroutine"]
    C3 --> G3["Goroutine"]
    CN --> GN["Goroutine"]

    G1 --> STORE[("Shared Data Store")]
    G2 --> STORE
    G3 --> STORE
    GN --> STORE
```

This allows multiple clients to interact with the server concurrently without creating a separate process for each connection.

---

## RESP Protocol

MiniRedis implements the core concepts of the **Redis Serialization Protocol (RESP)**.

A command such as:

```text
SET name Sahil
```

is transmitted in RESP form:

```text
*3
$3
SET
$4
name
$5
Sahil
```

The server parses the byte stream, converts it into a structured command, executes it, and serializes the result back into RESP.

```mermaid
flowchart LR
    RAW["Raw TCP Bytes"] --> PARSE["RESP Parser"]
    PARSE --> CMD["Structured Command"]
    CMD --> EXEC["Command Execution"]
    EXEC --> RESP["RESP Response"]
    RESP --> CLIENT["Client"]
```

---

## Supported Data Types

### Strings

Basic key-value operations:

```text
SET name Sahil
GET name
DEL name
```

### Hashes

Store multiple fields under a single key:

```text
HSET user name Sahil
HGET user name
```

The command set can be extended as the database evolves.

---

## Project Structure

```text
.
├── main.go          # Server initialization and startup
├── handler.go       # Client handling and command execution
├── resp.go          # RESP protocol reader/writer
├── aof.go           # Append Only File persistence
├── go.mod           # Go module definition
└── README.md
```

The implementation intentionally keeps the codebase small so that each major database component can be understood independently.

---

## Getting Started

### Requirements

* Go 1.20+
* Git
* Optional: `redis-cli`

### Clone

```bash
git clone https://github.com/<your-username>/miniredis.git
cd miniredis
```

### Run

```bash
go run .
```

### Build

```bash
go build -o miniredis .
```

Run the compiled binary:

```bash
./miniredis
```

---

## Usage

If the server is running on the default Redis port, connect using:

```bash
redis-cli
```

Then execute supported commands:

```text
127.0.0.1:6379> SET name Sahil
OK

127.0.0.1:6379> GET name
"Sahil"

127.0.0.1:6379> HSET user name Sahil
(integer) 1

127.0.0.1:6379> HGET user name
"Sahil"
```

---

## Design Goals

MiniRedis was built to understand the internal mechanics of a database server.

The project explores:

* TCP networking
* Client-server architecture
* Binary/text protocol design
* Protocol parsing
* Serialization
* Concurrent request handling
* In-memory data structures
* Command dispatch
* Persistence
* Crash recovery
* Database server architecture

Rather than hiding these concepts behind libraries, MiniRedis implements the core components directly.

---

## Roadmap

Potential future improvements:

* [ ] Additional Redis commands
* [ ] Lists
* [ ] Sets
* [ ] Sorted sets
* [ ] TTL and key expiration
* [ ] Transactions
* [ ] Pub/Sub
* [ ] Request pipelining
* [ ] AOF rewriting
* [ ] Graceful shutdown
* [ ] Configuration system
* [ ] Automated unit and integration tests
* [ ] Benchmarking
* [ ] Improved concurrency and synchronization
* [ ] Memory usage optimizations

---

## Tech Stack

```text
Language       Go
Networking     TCP
Protocol       RESP
Persistence    Append Only File
Concurrency    Goroutines
Storage        In-Memory
Client         redis-cli / RESP-compatible clients
```

---

## What This Project Demonstrates

MiniRedis demonstrates practical experience with low-level backend and systems concepts including:

* Network programming
* Concurrent server design
* Protocol implementation
* Data structure design
* Persistence mechanisms
* State recovery
* Request/response architectures
* Go concurrency primitives
* Database internals

---

## License

* [ ] This project is licensed under the MIT License.
