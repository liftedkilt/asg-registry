# Auto-Scaling Group Identifier Service

This repository contains a lightweight API service designed to manage unique identifiers for virtual machines (VMs) in an **AWS Auto-Scaling Group (ASG)**. The service ensures that each VM is assigned a unique identifier, tracks their liveness, and reclaims identifiers when VMs become unresponsive.

---

## 🚀 Features

- Assigns **unique identifiers** to VMs on startup.
- Tracks VM liveness through periodic probes.
- Reclaims identifiers from stale or unresponsive VMs.
- Prevents identifier reuse conflicts.
- Provides robust and clear error responses for invalid operations.

---

## 🛠️ Setup and Installation

### Prerequisites

- **Go**: Version 1.22.10 or higher.
- **SQLite**: Used as the underlying database.
- **AWS Auto-Scaling Group**: VM hostnames are used as unique `client_id`s.

### Install Dependencies
```
go mod tidy
```

### Database Setup
The service uses an SQLite database to store and manage identifiers. On the first run, the database and required tables will be created automatically.

## 📖 API Documentation

### 1️⃣ /allocate

Description: Allocates a unique identifier to a VM.

Method: POST

Payload:
```
{
  "client_id": "vm-hostname"
}
```

Response:
```
{
  "identifier": "unique-identifier"
}
```

Errors:

`503 Service Unavailable`: No identifiers are available.


#### 2️⃣ /liveness
Description: Updates the last-seen timestamp for a VM.

Method: POST

Payload:
```
{
  "client_id": "vm-hostname",
  "identifier": "unique-identifier"
}
```
Response:

`200 OK`: Liveness updated successfully.

Errors:
`404 Not Found`: The identifier does not exist.
`409 Conflict`: The client_id does not own the specified identifier.

#### 3️⃣ /identifiers
Description: Lists all identifiers and their allocation status.

Method: GET

Response:
```
[
  {
    "identifier": "unique-identifier",
    "client_id": "vm-hostname",
    "last_seen": "2024-01-08T10:00:00Z"
  },
  {
    "identifier": "unique-identifier-2",
    "client_id": null,
    "last_seen": null
  }
]
```

### 4️⃣ /client/`{client_id}`

Description: Retrieves details about a specific client.

Method: GET

Response:
```
{
  "client_id": "vm-hostname",
  "identifier": "unique-identifier",
  "last_seen": "2024-01-08T10:00:00Z"
}
```

### /identifier/{identifier}
Description: Retrieves details about a specific identifier.

Method: GET

Response:
```
{
  "client_id": "vm-hostname",
  "identifier": "unique-identifier",
  "last_seen": "2024-01-08T10:00:00Z"
}
```

### 6️⃣ /health
Description: Health check endpoint to verify service status.

Method: GET

Response:
```
{
  "status": "healthy",
  "uptime": "2h34m"
}
```

⚙️ How It Works
1. Startup:
    - The service preloads a list of unique identifiers into the database.
1. VM Allocation: 
    - A VM sends a POST /allocate request with its client_id (hostname).
    - The service assigns the next available identifier.
1. Liveness Probes:
    - The VM periodically sends a POST /liveness request to maintain ownership of the identifier.
    - If the VM fails to send probes, the identifier is marked as stale and becomes available for reuse.
1. Conflict Handling:
    - If a VM sends a liveness probe for an identifier it does not own, the service responds with 409 Conflict.

### 🚦 Error Handling
|Error|HTTP Code|Description|
|---|---|---|
|Invalid JSON|400|Malformed request payload.|
|Identifier not found|404|The specified identifier does not exist.|
|No available identifiers|503|No identifiers are available for allocation.|
|Identifier mismatch|409|The client_id does not match the owner of the identifier.|
