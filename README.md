# go-comu-bin

Single-binary community backend written in Go.

`go-comu-bin`은 기능 구현 자체보다, 확장 가능한 구조(레이어드 + 포트/어댑터)와 운영 가능한 API 경계 설계를 목표로 하는 프로젝트입니다.

## Highlights

- Layered + Hexagonal Architecture (`delivery -> application -> domain -> infrastructure`)
- JWT + session validation authentication flow
- Role/owner-based authorization policy
- Report domain + admin moderation APIs
- Dead outbox operations (`list`, `requeue`, `discard`)
- Hidden board visibility policy (non-admin 완전 비노출)
- Cursor pagination (`limit`, `last_id`) with max page limit guard (`1..1000`)
- OpenAPI/Swagger generation and verification pipeline

## Architecture

핵심 원칙:

- Domain-centric design
- Port/Adapter separation
- `context.Context` first at boundary methods
- Application-layer orchestration + policy
- Infrastructure adapters for persistence/cache/auth/storage

자세한 내용은 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) 참고.

## Quick Start

Prerequisites:

- Go 1.22+ (권장)
- `make`

1. 설정 파일 준비

```bash
cp config.yml config.local.yml
```

`config.yml` 또는 `config/config.yml` 경로를 사용하며, 최소한 `delivery.http.auth.secret`는 32자 이상 실제 값으로 설정해야 합니다.

2. 실행

```bash
make run
```

또는:

```bash
go run ./cmd
```

3. Swagger 확인

- [http://localhost:18577/swagger/index.html](http://localhost:18577/swagger/index.html)

## Configuration

주요 설정:

- `delivery.http.port`
- `delivery.http.maxJSONBodyBytes`
- `delivery.http.defaultPageLimit` (`1..1000`)
- `delivery.http.auth.secret` (최소 32자)
- `event.outbox.*`
- `storage.*`
- `admin.bootstrap.*`
- `jobs.*`

상세 규칙/예시는 [docs/CONFIG.md](docs/CONFIG.md) 참고.

## API

- Base path: `/api/v1`
- OpenAPI docs: `docs/swagger/`
- Human-readable API guide: [docs/API.md](docs/API.md)

주요 엔드포인트 그룹:

- Auth/User
- Board/Post/Comment/Reaction
- Attachment
- Report/Admin Operations

## Project Structure

```text
cmd/                      # composition root (wiring)
internal/
  application/            # use case, service, policy, ports
  domain/                 # entities and domain rules
  delivery/               # HTTP adapters
  infrastructure/         # adapters (inmemory/cache/auth/storage/event)
docs/                     # architecture, API, config, roadmap, decisions
```

## Development

```bash
make help
make build
make run
make test
make swagger
make verify
```

- `make verify` runs tests, `go vet`, and swagger sync verification.
- 필요 시 로컬 훅 설치: `./scripts/install-githooks.sh`

## Testing

```bash
go test ./...
```

패키지별/스타일 가이드는 [docs/TESTING.md](docs/TESTING.md) 참고.

## Roadmap and Decisions

- Roadmap: [docs/ROADMAP.md](docs/ROADMAP.md)
- Decision log: [docs/DECISIONS.md](docs/DECISIONS.md)

프로젝트의 정책/범위 변경은 DECISIONS를 기준으로 추적합니다.

## Contributing

이 저장소는 문서-우선 변경 흐름을 사용합니다.

- 결정 기록 -> TDD -> 구현 -> 테스트 -> 문서 정합성

큰 변경 전에는 관련 `docs/`와 결정 기록을 먼저 확인해 주세요.

## License

Currently no license file is defined in this repository.
