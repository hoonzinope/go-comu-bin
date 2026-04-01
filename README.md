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
- Public cursor pagination (`limit`, `cursor`) with max page limit guard (`1..1000`)
- HTML-first Web UI / Admin Console with SSR fallback
- Playwright e2e + visual regression coverage
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

## Docker

Docker Hub에 올릴 때는 다중 아키텍처 이미지를 빌드하는 쪽이 안전합니다.

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t docker.io/<your-user>/commu-bin:latest \
  --push .
```

Mac mini에서 컨테이너를 띄울 때는 SQLite DB와 로컬 업로드 경로를 호스트 바인드 마운트 대신 named volume으로 붙이세요.

```bash
docker run -d \
  --name commu-bin \
  -p 18577:18577 \
  -e DELIVERY_HTTP_AUTH_SECRET='replace-with-real-secret' \
  -v commu_data:/app/data \
  -v commu_logs:/app/logs \
  docker.io/<your-user>/commu-bin:latest
```

- `/app/data` 아래에 SQLite DB와 로컬 업로드가 함께 들어갑니다.
- `/app/logs` 아래에 파일 로그가 기록됩니다.
- macOS 호스트 경로를 직접 붙이는 bind mount는 SQLite WAL과 충돌할 수 있으니 피하는 편이 낫습니다.

`docker-compose.yml`을 쓰면 같은 구성을 더 짧게 반복할 수 있습니다.
호스트에서 `config.yml`을 관리하려면 `COMMU_BIN_CONFIG_PATH`로 파일 경로를 지정해 `/app/config.yml`에 읽기 전용 마운트합니다.
기본 `config.yml`에는 로컬 관리용 초기 bootstrap admin(`admin` / `commu-admin-1q2w#E$R!`)이 들어 있습니다.

`.env.example`을 `.env`로 복사한 뒤 값을 채우면 됩니다.

```bash
COMMU_BIN_IMAGE=docker.io/<your-user>/commu-bin:latest \
COMMU_BIN_CONFIG_PATH=/Users/hoonzi/Documents/docker_v/go-commu-bin-data/config.yml \
DELIVERY_HTTP_AUTH_SECRET='replace-with-real-secret' \
docker compose up -d
```

- `commu_data`와 `commu_logs` named volume이 자동으로 생성됩니다.
- `COMMU_BIN_CONFIG_PATH`가 가리키는 호스트 `config.yml`이 `/app/config.yml`로 마운트됩니다.
- bootstrap admin은 `config.yml`의 `admin.bootstrap.*` 값으로 시드됩니다.
- 관리용 점검은 `scripts/check-bootstrap-admin.sh`를 사용하면 됩니다.
- 이미지 이름은 Docker Hub에 push한 태그로 바꾸면 됩니다.

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
- OpenAPI docs: `docs/swagger/` (`make swagger`로 갱신, `make verify-swagger`로 정합성 확인)
- Human-readable API guide: [docs/API.md](docs/API.md)

주요 엔드포인트 그룹:

- Auth/User
- Notification
- Board/Post/Comment/Reaction
- Attachment
- Report/Admin Operations

## Web UI

- Browser UI routes live outside `/api/v1`.
- SSR HTML + Alpine.js shell is served from the web delivery layer.
- Browser routes cover feed, post detail, composer, auth/account, notifications, my page, and admin console flows.
- The implementation lives in `internal/delivery/web`.

## Project Structure

```text
cmd/                      # composition root (wiring)
internal/
  application/            # use case, service, policy, ports
  domain/                 # entities and domain rules
  delivery/               # transport adapters
    api/                  # JSON API delivery
    web/                  # SSR HTML UI/admin console
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
npm run test:e2e:chromium
npm run test:e2e:visual
```

- `make verify` runs tests, `go vet`, and swagger sync verification.
- Playwright visual baselines can be refreshed with `npm run test:e2e:visual:update` after intentional UI changes.
- 필요 시 로컬 훅 설치: `./scripts/install-githooks.sh`

## Testing

```bash
go test ./...
npm run test:e2e
npm run test:e2e:chromium
npm run test:e2e:visual
```

Browser E2E/visual tests require Node.js/npm and the Playwright browsers installed.
Swagger 산출물은 `docs/swagger/docs.go`, `docs/swagger/swagger.json`, `docs/swagger/swagger.yaml`로 생성됩니다.

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
