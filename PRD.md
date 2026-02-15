# Product Requirements Document: Go Monitor

## Overview
A high-performance e-commerce monitoring system built in Go that tracks product availability across multiple platforms and provides real-time notifications via Discord webhooks. This is a complete rewrite of the TypeScript Flipz Monitor with improved reliability, performance, and observability.

## Goals
- **Learn Go**: Practical application of goroutines, channels, interfaces, and generics
- **Improve Reliability**: Smarter error handling and automatic recovery
- **Better Observability**: Full analytics database and Prometheus metrics
- **Production Ready**: Deployable system with clean architecture

---

## MVP Scope

### Supported Platforms (4)
1. **Shopify** - Restock + Search monitoring
2. **SquareSpace** - Restock + Search monitoring
3. **Big Cartel** - Restock + Search monitoring
4. **Reddit** - New post monitoring only

### Core Features

#### 1. Task Management
- **Task Types:**
  - `RestockTask`: Monitor single product URL for stock changes
  - `SearchTask`: Scan collection/category pages for new products
  - `RedditTask`: Monitor subreddit for new posts

- **Task Configuration:**
  - Platform type
  - Target URL(s)
  - Check interval (customizable per task)
  - Webhook URL for notifications
  - Enabled/disabled state

#### 2. Error Handling & Recovery
- **Exponential Backoff Strategy:**
  ```
  Error Count | Delay
  -----------|--------
  1          | 1 min
  2          | 2 min
  3          | 4 min
  4          | 8 min
  5          | 16 min
  6+         | 30 min (max)
  ```
- **Auto-Recovery:** Tasks automatically resume with adjusted intervals
- **Pattern Detection:** Learn platform-specific error patterns and adapt
- **Never Die:** Tasks slow down but never permanently stop

#### 3. Discord Bot Integration
**Single Binary Architecture:** Discord bot and monitor run in same process

**Command Set (All from existing bot):**
- `/add` - Create new monitoring task
- `/remove` - Delete a task
- `/list` - Show all tasks (concise view: ✅ Shopify - store.com/product)
- `/start` - Enable a task
- `/stop` - Disable a task
- `/restart` - Reset task error state
- `/status` - Show detailed task health
- `/config` - View/update task settings
- `/webhook` - Manage webhook URLs
- `/settings` - Global configuration

**Task Display Format (Concise):**
```
✅ Shopify - sneakers.myshopify.com/products/jordan-1
⏸️  BigCartel - artist.bigcartel.com/search
❌ SquareSpace - gallery.squarespace.com/shop (3 errors)
🔄 Reddit - r/mechmarket (checking...)
```

#### 4. Webhook Notifications
- **Restock Tasks:** Ping webhook when stock status changes (in stock ↔ out of stock)
- **Search Tasks:** Ping webhook when new products discovered
- **Reddit Tasks:** Ping webhook when new posts found
- **Error Notifications:** Separate webhook for critical errors (5+ consecutive failures)

#### 5. Database (Full Analytics)
**Purpose:** Replace JSON files + provide rich analytics for future dashboard

**Schema (PostgreSQL):**

```sql
-- Webhook configurations
CREATE TABLE webhooks (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  url TEXT NOT NULL UNIQUE,
  webhook_type VARCHAR(20) NOT NULL,          -- 'normal', 'error'
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Proxy list groups
CREATE TABLE proxy_lists (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Individual proxies
CREATE TABLE proxies (
  id SERIAL PRIMARY KEY,
  proxy_list_id INT NOT NULL REFERENCES proxy_lists(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE(proxy_list_id, url)
);

CREATE INDEX idx_proxies_proxy_list_id ON proxies(proxy_list_id);

-- Authorized Discord users
CREATE TABLE discord_users (
  id SERIAL PRIMARY KEY,
  discord_user_id VARCHAR(20) NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Task configurations
CREATE TABLE tasks (
  id SERIAL PRIMARY KEY,
  platform VARCHAR(50) NOT NULL,
  task_type VARCHAR(20) NOT NULL,              -- 'restock', 'search', 'reddit'
  url TEXT NOT NULL,
  webhook_id INT REFERENCES webhooks(id),
  proxy_list_id INT REFERENCES proxy_lists(id),
  delay INT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_webhook_id ON tasks(webhook_id);
CREATE INDEX idx_tasks_proxy_list_id ON tasks(proxy_list_id);

-- Task execution history
CREATE TABLE task_runs (
  id SERIAL PRIMARY KEY,
  task_id INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  started_at TIMESTAMP NOT NULL,
  completed_at TIMESTAMP,
  status VARCHAR(20) NOT NULL,                  -- 'success', 'error', 'timeout'
  error_message TEXT
);

CREATE INDEX idx_task_runs_task_id ON task_runs(task_id);

-- Items (products, posts, etc.) - generic for all platforms
CREATE TABLE items (
  id SERIAL PRIMARY KEY,
  task_id INT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  platform VARCHAR(50) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  data JSONB NOT NULL DEFAULT '{}',             -- Flexible storage: title, price, in_stock, etc.
  UNIQUE(task_id, url)
);

CREATE INDEX idx_items_task_id ON items(task_id);

-- Item state change events (stock, price, etc.)
CREATE TABLE item_events (
  id SERIAL PRIMARY KEY,
  item_id INT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  previous_state JSONB NOT NULL,                -- Full previous item.data
  new_state JSONB NOT NULL,                     -- Full new item.data
  webhook_sent BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_item_events_item_id ON item_events(item_id);
```

**Key Design Decisions:**
- **SERIAL vs UUID**: Auto-incrementing integers for simplicity and performance
- **JSONB for flexibility**: `items.data` and `item_events` use JSONB to handle platform-specific fields
- **Generic items table**: Handles both products (Shopify) and posts (Reddit) using flexible JSONB
- **Separate webhooks table**: Reusable webhooks across tasks, separate error webhooks
- **Proxy lists**: Named groups of proxies assignable to tasks
- **No response_time_ms**: Calculated from `completed_at - started_at`
- **No settings table**: Use environment variables instead
- **No errors table**: Errors tracked in `task_runs.error_message`

---

## Technical Architecture

### Project Structure
```
go-monitor/
├── cmd/
│   └── monitor/
│       └── main.go              # Entry point
├── migrations/                  # SQL migrations (root level)
│   ├── 00001_init.up.sql
│   └── 00001_init.down.sql
├── internal/
│   ├── monitor/
│   │   ├── task.go              # Task interface + manager
│   │   ├── scheduler.go         # Goroutine orchestration
│   │   ├── platforms/
│   │   │   ├── shopify/
│   │   │   │   ├── restock.go
│   │   │   │   └── search.go
│   │   │   ├── squarespace/
│   │   │   ├── bigcartel/
│   │   │   └── reddit/
│   │   └── retry.go             # Exponential backoff logic
│   ├── discord/
│   │   ├── bot.go               # Discord client
│   │   ├── commands/
│   │   │   ├── add.go
│   │   │   ├── list.go
│   │   │   └── ...
│   │   └── handlers/            # Command execution
│   ├── database/
│   │   ├── db.go                # Database client
│   │   ├── models.go            # sqlc generated types
│   │   ├── tasks.sql.go         # sqlc generated queries
│   │   └── queries/             # SQL query definitions
│   │       └── tasks.sql
│   ├── webhook/
│   │   └── sender.go            # Webhook delivery
│   └── config/
│       └── config.go            # Configuration management
├── pkg/
│   └── scraper/                 # Shared HTTP/scraping utilities
├── go.mod
├── go.sum
├── PRD.md                       # This file
├── README.md
└── .gitignore
```

### Core Interfaces

#### Task Interface
```go
type Task interface {
    ID() string
    Platform() Platform
    Type() TaskType
    Run(ctx context.Context) (*TaskResult, error)
    Interval() time.Duration
    IsEnabled() bool
}

type TaskResult struct {
    Success      bool
    ProductsFound int
    StockChanges  int
    NewProducts   []Product
    Error        error
}
```

#### Platform Enum
```go
type Platform string

const (
    PlatformShopify      Platform = "shopify"
    PlatformSquareSpace  Platform = "squarespace"
    PlatformBigCartel    Platform = "bigcartel"
    PlatformReddit       Platform = "reddit"
)
```

#### Task Types
```go
type TaskType string

const (
    TaskTypeRestock TaskType = "restock"
    TaskTypeSearch  TaskType = "search"
    TaskTypeReddit  TaskType = "reddit"
)
```

### Concurrency Model

**Task Scheduler:**
- Main goroutine manages task lifecycle
- Each task runs in its own goroutine
- Channels for task control (start, stop, restart)
- Context cancellation for graceful shutdown

```go
// Pseudo-code
func (s *Scheduler) Start(ctx context.Context) {
    for _, task := range s.tasks {
        go s.runTask(ctx, task)
    }

    // Listen for control signals
    for {
        select {
        case <-ctx.Done():
            return
        case cmd := <-s.controlChan:
            s.handleCommand(cmd)
        }
    }
}

func (s *Scheduler) runTask(ctx context.Context, task Task) {
    ticker := time.NewTicker(task.Interval())
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.executeTask(ctx, task)
        }
    }
}
```

### Error Handling

**Exponential Backoff Implementation:**
```go
type RetryManager struct {
    taskErrors map[string]*ErrorState
}

type ErrorState struct {
    Count         int
    LastError     time.Time
    NextRetry     time.Time
    CurrentDelay  time.Duration
}

func (rm *RetryManager) RecordError(taskID string) time.Duration {
    state := rm.taskErrors[taskID]
    state.Count++

    // Exponential: 1min, 2min, 4min, 8min, 16min, 30min (max)
    delay := time.Minute * time.Duration(math.Pow(2, float64(state.Count-1)))
    if delay > 30*time.Minute {
        delay = 30*time.Minute
    }

    state.CurrentDelay = delay
    state.NextRetry = time.Now().Add(delay)

    return delay
}
```

---

## Implementation Plan

### Phase 1: Foundation (Week 1)
- [ ] Project setup (go.mod, structure)
- [ ] Database schema + migrations
- [ ] Task interface + base types
- [ ] Configuration management

### Phase 2: Core Monitor (Week 2)
- [ ] Task scheduler with goroutines
- [ ] Retry/backoff system
- [ ] Webhook sender
- [ ] Implement Shopify restock task

### Phase 3: Platform Implementations (Week 2-3)
- [ ] Shopify search task
- [ ] SquareSpace restock + search
- [ ] Big Cartel restock + search
- [ ] Reddit monitoring

### Phase 4: Discord Bot (Week 3)
- [ ] Discord client setup
- [ ] Command registration
- [ ] Core commands (add, remove, list, start, stop)
- [ ] Management commands (restart, status, config, webhook)

### Phase 5: Analytics & Polish (Week 4)
- [ ] Database analytics queries
- [ ] Error tracking and logging
- [ ] Performance optimization
- [ ] Documentation

---

## Success Metrics

**Reliability:**
- < 1% task failure rate after retries
- Auto-recovery from temporary platform outages
- Zero manual restarts required

**Performance:**
- Task execution < 5 seconds (p95)
- Support 50+ concurrent tasks
- Memory usage < 100MB

**Observability:**
- All task runs logged to database
- Error patterns identified automatically
- Dashboard-ready metrics available

---

## Future Enhancements (Post-MVP)

1. **Prometheus Metrics**
   - Expose /metrics endpoint
   - Track request rates, error rates, latency

2. **Web Dashboard**
   - Real-time task monitoring
   - Historical analytics graphs
   - Task management UI

3. **Additional Platforms**
   - Amazon, Target, Urban Outfitters
   - Music stores (MusicToday, RoughTrade, etc.)

4. **Advanced Features**
   - Price tracking and alerts
   - Multi-region proxy support
   - Rate limit auto-detection
   - Machine learning for optimal check intervals

---

## Technical Decisions

### Why PostgreSQL?
- Rich analytics queries for dashboard
- JSONB support for flexible metadata
- Production-ready scaling path
- (SQLite acceptable for learning/testing)

### Why Single Binary?
- Simpler deployment (one process)
- Lower latency (no network hop)
- Easier debugging
- Can split later if needed

### Why Exponential Backoff?
- Automatic adaptation to platform issues
- Reduces unnecessary load during outages
- Prevents permanent task death
- Industry-standard retry pattern

### Why SERIAL over UUID?
- Smaller storage (4 bytes vs 16 bytes)
- Faster joins and indexes
- Easier debugging (task #42 vs 550e8400-e29b-41d4...)
- Sequential ordering
- Single database instance (don't need global uniqueness)

### Why JSONB for items.data?
- Platform flexibility (Shopify has different fields than Reddit)
- No schema changes when adding new platforms
- Can store varying fields (price, currency, images, etc.)
- Still queryable with PostgreSQL JSONB operators
- Simpler webhook payload construction

### Why sqlc over ORM?
- Type-safe SQL at compile time
- Write actual SQL (learn properly, full control)
- No hidden queries or N+1 problems
- Lightweight (generates code, not runtime overhead)
- Go-idiomatic (explicit, not magical)

### Go Libraries (Recommended)
- `discordgo` - Discord API
- `goquery` - HTML parsing (like cheerio)
- `sqlc` - Type-safe SQL queries
- `migrate` - Database migrations
- `zap` - Structured logging
- `viper` - Configuration management

---

## Questions Resolved

1. **Proxy Support**: ✅ Yes - proxy_lists and proxies tables, assignable per task
2. **Authentication**: ✅ discord_users table for authorized users
3. **Webhooks**: ✅ Separate webhooks table, reusable across tasks
4. **Database IDs**: ✅ SERIAL (auto-increment) instead of UUID
5. **Item Storage**: ✅ Generic `items` table with JSONB for flexibility
6. **Settings**: ✅ Use environment variables, no settings table

## Questions Still Open

1. **Anti-Bot**: CloudScraper equivalent needed for Go?
2. **Rate Limiting**: Per-platform rate limits in MVP or post-MVP?
3. **Deployment**: Docker container or binary distribution?

