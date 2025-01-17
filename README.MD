# Dcard 2021 Web Backend Intern Homework

## 作業題目

 - Dcard 每天午夜都有大量使用者湧入抽卡，為了不讓伺服器過載，請設計一個 middleware：
 - 限制每小時來自同一個 IP 的請求數量不得超過 1000
 - 在 response headers 中加入剩餘的請求數量 (X-RateLimit-Remaining) 以及 rate limit 歸零的時間 (X-RateLimit-Reset)
 - 如果超過限制的話就回傳 429 (Too Many Requests)
 - 可以使用各種資料庫達成

## Run

```bash
go run main.go
```

### Redis (in docker)

```bash
docker run -p 6379:6397 redis:alpine
```

## Tests

```bash
go test ./...
```

### With Code Coverage

```bash
go test ./... -cover
```

### Benchmark

```bash
go test ./... -bench .
```
