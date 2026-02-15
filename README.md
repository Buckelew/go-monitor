# Go Monitor
A product monitoring service written in Go.

## Setup

### Pre Requisites
- Go 1.25.7
- PostgreSQL 18

### Developer setup

1. Copy `.env.example` to `.env` and fill in the required values. Ask Caden or Nate for the keys.

```bash
cp .env.example .env
nano .env
```

2. Create the databases.

```bash
createdb go_monitor
createdb go_monitor_test
```

3. Run migrations.
