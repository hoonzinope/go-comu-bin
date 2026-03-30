# 설정 (Config)

## 로딩 위치

서버 시작 시 아래 순서로 설정 파일을 탐색합니다.

- `./config.yml`
- `./config/config.yml`

환경 변수도 함께 읽습니다. 설정 파일이 없어도 환경 변수만으로 부팅할 수 있습니다.

- 예: `DELIVERY_HTTP_AUTH_SECRET`
- 예: `ADMIN_BOOTSTRAP_ENABLED`
- 예: `ADMIN_BOOTSTRAP_USERNAME`
- 예: `ADMIN_BOOTSTRAP_PASSWORD`
- 예: `LOGGING_FILEPATH`

Docker Compose로 호스트 설정 파일을 쓰는 경우에는 `COMMU_BIN_CONFIG_PATH`를 `/app/config.yml`에 마운트해서 사용합니다.
기본 `config.yml`은 로컬 관리용 초기 bootstrap admin(`admin` / `commu-admin-1q2w#E$R!`)을 포함합니다.

## 예시

```yaml
cache:
  listTTLSeconds: 30
  detailTTLSeconds: 30

logging:
  filePath: "./logs/app.jsonl"
  maxSizeMB: 100
  maxBackups: 10
  maxAgeDays: 30
  compress: true
  localTime: true

database:
  path: "./data/data.db"

storage:
  provider: "local"
  local:
    rootDir: "./data/uploads"
  object:
    endpoint: ""
    bucket: ""
    accessKey: ""
    secretKey: ""
    useSSL: false
  attachment:
    maxUploadSizeBytes: 10485760
    imageOptimization:
      enabled: true
      jpegQuality: 82

delivery:
  http:
    port: 18577
    maxJSONBodyBytes: 1048576
    defaultPageLimit: 10
    trustedProxies:
      - "127.0.0.1"
      - "10.0.0.0/8"
    rateLimit:
      enabled: true
      windowSeconds: 60
      readRequests: 300
      writeRequests: 60
    auth:
      secret: "replace-with-real-secret"
      loginRateLimit:
        enabled: true
        windowSeconds: 60
        maxRequests: 5
      guestUpgradeRateLimit:
        enabled: true
        windowSeconds: 60
        maxRequests: 5
      emailVerificationRequestRateLimit:
        enabled: true
        windowSeconds: 60
        maxRequests: 5
      passwordResetRequestRateLimit:
        enabled: true
        windowSeconds: 60
        maxRequests: 5
  mail:
    enabled: false
    emailVerification:
      baseURL: "https://app.example.com/verify-email"
    passwordReset:
      baseURL: "https://app.example.com/reset-password"
    smtp:
      host: "smtp.example.com"
      port: 587
      username: ""
      password: ""
      from: "noreply@example.com"
      startTLS: true
      implicitTLS: false

event:
  outbox:
    workerCount: 1
    batchSize: 100
    pollIntervalMillis: 100
    maxAttempts: 5
    baseBackoffMillis: 200
    processingLeaseMillis: 30000
    leaseRefreshMillis: 10000

admin:
  bootstrap:
    enabled: false
    username: ""
    password: ""

jobs:
  enabled: true
  attachmentCleanup:
    enabled: true
    intervalSeconds: 600
    gracePeriodSeconds: 600
    batchSize: 50
  guestCleanup:
    enabled: true
    intervalSeconds: 600
    pendingGracePeriodSeconds: 600
    activeUnusedGracePeriodSeconds: 86400
    batchSize: 50
  emailVerificationCleanup:
    enabled: true
    intervalSeconds: 600
    gracePeriodSeconds: 600
    batchSize: 100
  passwordResetCleanup:
    enabled: true
    intervalSeconds: 600
    gracePeriodSeconds: 600
    batchSize: 100
```

## 로깅

- 기본 로그 포맷은 JSON이다.
- runtime 로그는 `stdout`과 회전 파일에 동시에 기록한다.
- 회전 정책은 `logging.*`에서 제어한다.
  - `filePath`
  - `maxSizeMB`
  - `maxBackups`
  - `maxAgeDays`
  - `compress`
  - `localTime`
- panic/recover 경계는 HTTP 미들웨어, background job runner, outbox relay worker, process entrypoint에서 구조화 로그를 남긴다.

## 리버스 프록시

- 앱이 프록시 뒤에서 동작하면 `delivery.http.trustedProxies`에 해당 프록시의 IP 또는 CIDR을 지정한다.
- 기본값은 비워 두며, 이 경우 `X-Forwarded-For` 같은 헤더를 신뢰하지 않는다.
- `nginx`, `Caddy`, 로드밸런서처럼 TLS 종료를 담당하는 프록시가 있으면 앱 내부 HTTPS 종료는 필요하지 않다.

## 검증 규칙

- `delivery.http.port`: `1..65535`
- `delivery.http.maxJSONBodyBytes`: `> 0`
- `delivery.http.defaultPageLimit`: `1..1000`
- `delivery.http.rateLimit.windowSeconds`: `>= 1`
- `delivery.http.rateLimit.readRequests`: `>= 1`
- `delivery.http.rateLimit.writeRequests`: `>= 1`
- `delivery.http.auth.secret`: 필수(빈 값 불가)
- `delivery.http.auth.secret`: placeholder 값 금지 (`commu-bin-secret-key`)
- `delivery.http.auth.secret`: 최소 길이 `32`자 이상
- `delivery.http.auth.loginRateLimit.windowSeconds`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.loginRateLimit.maxRequests`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.guestUpgradeRateLimit.windowSeconds`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.guestUpgradeRateLimit.maxRequests`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.emailVerificationRequestRateLimit.windowSeconds`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.emailVerificationRequestRateLimit.maxRequests`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.passwordResetRequestRateLimit.windowSeconds`: `enabled=true`일 때 `>= 1`
- `delivery.http.auth.passwordResetRequestRateLimit.maxRequests`: `enabled=true`일 때 `>= 1`
- `delivery.mail.emailVerification.baseURL`: `delivery.mail.enabled=true`일 때 필수
- `delivery.mail.passwordReset.baseURL`: `delivery.mail.enabled=true`일 때 필수
- `delivery.mail.smtp.host`: `delivery.mail.enabled=true`일 때 필수
- `delivery.mail.smtp.port`: `delivery.mail.enabled=true`일 때 `1..65535`
- `delivery.mail.smtp.from`: `delivery.mail.enabled=true`일 때 필수
- `delivery.mail.smtp.startTLS`와 `delivery.mail.smtp.implicitTLS`: 동시에 `true` 불가
- `event.outbox.workerCount`: `> 0`
- `event.outbox.batchSize`: `> 0`
- `event.outbox.pollIntervalMillis`: `> 0`
- `event.outbox.maxAttempts`: `> 0`
- `event.outbox.baseBackoffMillis`: `> 0`
- `event.outbox.processingLeaseMillis`: `> 0`
- `event.outbox.leaseRefreshMillis`: `> 0`
- `event.outbox.leaseRefreshMillis`: `processingLeaseMillis`보다 작아야 함
- `admin.bootstrap.enabled`: 기본 `true` in repo sample config
- `admin.bootstrap.username`: bootstrap enabled일 때 필수
- `admin.bootstrap.password`: bootstrap enabled일 때 필수
- `cache.listTTLSeconds`: `> 0`
- `cache.detailTTLSeconds`: `> 0`
- `cache.maxCost`: `> 0`, runtime cache capacity
- `cache.numCounters`: `> 0`, admission/eviction frequency counters
- `cache.bufferItems`: `> 0`, buffered write/get size
- `cache.metrics`: `true | false`, cache metrics on/off
- `storage.local.rootDir`: 필수(빈 값 불가)
- `database.path`: SQLite auth DB 파일 경로, 필수
- `storage.provider`: `local | object`
- `storage.object.endpoint`: provider가 `object`일 때 필수
- `storage.object.bucket`: provider가 `object`일 때 필수
- `storage.object.accessKey`: provider가 `object`일 때 필수
- `storage.object.secretKey`: provider가 `object`일 때 필수
- `storage.attachment.maxUploadSizeBytes`: `> 0`
- `storage.attachment.imageOptimization.jpegQuality`: `1..100`
- `jobs.attachmentCleanup.intervalSeconds`: `jobs.enabled=true` 이고 `jobs.attachmentCleanup.enabled=true`일 때 `> 0`
- `jobs.attachmentCleanup.gracePeriodSeconds`: `jobs.enabled=true` 이고 `jobs.attachmentCleanup.enabled=true`일 때 `> 0`
- `jobs.attachmentCleanup.batchSize`: `jobs.enabled=true` 이고 `jobs.attachmentCleanup.enabled=true`일 때 `> 0`
- `jobs.guestCleanup.intervalSeconds`: `jobs.enabled=true` 이고 `jobs.guestCleanup.enabled=true`일 때 `> 0`
- `jobs.guestCleanup.pendingGracePeriodSeconds`: `jobs.enabled=true` 이고 `jobs.guestCleanup.enabled=true`일 때 `> 0`
- `jobs.guestCleanup.activeUnusedGracePeriodSeconds`: `jobs.enabled=true` 이고 `jobs.guestCleanup.enabled=true`일 때 `> 0`
- `jobs.guestCleanup.batchSize`: `jobs.enabled=true` 이고 `jobs.guestCleanup.enabled=true`일 때 `> 0`
- `jobs.emailVerificationCleanup.intervalSeconds`: `jobs.enabled=true` 이고 `jobs.emailVerificationCleanup.enabled=true`일 때 `> 0`
- `jobs.emailVerificationCleanup.gracePeriodSeconds`: `jobs.enabled=true` 이고 `jobs.emailVerificationCleanup.enabled=true`일 때 `> 0`
- `jobs.emailVerificationCleanup.batchSize`: `jobs.enabled=true` 이고 `jobs.emailVerificationCleanup.enabled=true`일 때 `> 0`
- `jobs.passwordResetCleanup.intervalSeconds`: `jobs.enabled=true` 이고 `jobs.passwordResetCleanup.enabled=true`일 때 `> 0`
- `jobs.passwordResetCleanup.gracePeriodSeconds`: `jobs.enabled=true` 이고 `jobs.passwordResetCleanup.enabled=true`일 때 `> 0`
- `jobs.passwordResetCleanup.batchSize`: `jobs.enabled=true` 이고 `jobs.passwordResetCleanup.enabled=true`일 때 `> 0`
- 알 수 없는 키는 실패 처리 (`UnmarshalExact`)
  - 예: `delivery.http.prt` 오타는 서버 시작 실패

## 사용 위치

- 포트: `cmd/main.go` -> `cfg.Delivery.HTTP.Port`
- JSON body 최대 크기(bytes): `cmd/main.go` -> `cfg.Delivery.HTTP.MaxJSONBodyBytes`
  - JSON API 요청 바디가 이 값을 초과하면 `400 Bad Request (request body too large)`를 반환합니다.
- JWT 시크릿: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.Secret`
- HTTP read/write 요청 rate limit: `cmd/main.go` -> `cfg.Delivery.HTTP.RateLimit.*`
  - `enabled=true`일 때 `/api/v1` 하위 `GET/HEAD/OPTIONS` 요청은 `readRequests`, `POST/PUT/DELETE/PATCH` 요청은 `writeRequests`를 `method+route+client_ip` 기준으로 적용합니다.
- trusted proxies: `cmd/main.go` -> `cfg.Delivery.HTTP.TrustedProxies`
  - 앱이 reverse proxy 뒤에 있으면 해당 proxy의 IP 또는 CIDR을 지정합니다.
  - 기본값은 비워 두며, 이 경우 `X-Forwarded-For` 같은 전달 헤더를 신뢰하지 않습니다.
- login 전용 rate limit: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.LoginRateLimit.*`
  - `enabled=true`일 때 `POST /api/v1/auth/login`에 `login:client_ip:normalized_username` 기준 제한을 추가 적용합니다.
  - username 존재 여부, 비밀번호 오류, 로그인 성공 여부와 무관하게 동일하게 카운트합니다.
- guest upgrade 전용 rate limit: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.*`
  - `enabled=true`일 때 `POST /api/v1/auth/guest/upgrade`에 `guest-upgrade:user:<userID>:ip:<client_ip>` 기준 제한을 추가 적용합니다.
  - guest 여부, 입력 오류, token 오류, 성공 여부와 무관하게 동일하게 카운트합니다.
- password reset request 전용 rate limit: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.*`
  - `enabled=true`일 때 `POST /api/v1/auth/password-reset/request`에 `password-reset-request:client_ip:normalized_email` 기준 제한을 추가 적용합니다.
- email verification request 전용 rate limit: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.*`
  - `enabled=true`일 때 `POST /api/v1/auth/email-verification/request`에 `email-verification-request:user:<userID>` 기준 제한을 추가 적용합니다.
- email verification 메일 링크 base URL: `cmd/main.go` -> `cfg.Delivery.Mail.EmailVerification.BaseURL`
  - `delivery.mail.enabled=true`이면 필수이며, 메일 본문에 `${baseURL}?token=...` 링크를 생성합니다.
- password reset 메일 링크 base URL: `cmd/main.go` -> `cfg.Delivery.Mail.PasswordReset.BaseURL`
  - `delivery.mail.enabled=true`이면 필수이며, 메일 본문에 `${baseURL}?token=...` 링크를 생성합니다.
- outbox relay 워커 수: `cmd/main.go` -> `cfg.Event.Outbox.WorkerCount`
- outbox relay 배치 크기: `cmd/main.go` -> `cfg.Event.Outbox.BatchSize`
- outbox relay polling 주기(ms): `cmd/main.go` -> `cfg.Event.Outbox.PollIntervalMillis`
- outbox retry 최대 횟수: `cmd/main.go` -> `cfg.Event.Outbox.MaxAttempts`
- outbox retry base backoff(ms): `cmd/main.go` -> `cfg.Event.Outbox.BaseBackoffMillis`
- outbox processing lease(ms): `cmd/main.go` -> `cfg.Event.Outbox.ProcessingLeaseMillis`
- outbox lease refresh(ms): `cmd/main.go` -> `cfg.Event.Outbox.LeaseRefreshMillis`
  - 전달은 at-least-once이며, 실패 이벤트는 backoff 재시도 후 `dead` 상태로 남깁니다.
  - relay는 handler 처리 중 lease를 heartbeat로 갱신해 장시간 처리 중 stale reclaim으로 인한 중복 dispatch를 줄입니다.
- bootstrap admin: `cmd/main.go` -> `cfg.Admin.Bootstrap.*`
- 캐시 TTL 정책: `cmd/main.go` -> `cfg.Cache.ListTTLSeconds`, `cfg.Cache.DetailTTLSeconds`
  - runtime cache capacity: `cfg.Cache.MaxCost`, `cfg.Cache.NumCounters`, `cfg.Cache.BufferItems`, `cfg.Cache.Metrics`
- 로컬 업로드 루트: `cfg.Storage.Local.RootDir`
- SQLite auth DB 경로: `cfg.Database.Path`
- 파일 저장 provider: `cfg.Storage.Provider`
- object storage endpoint/bucket: `cfg.Storage.Object.Endpoint`, `cfg.Storage.Object.Bucket`
- attachment 최대 업로드 크기(bytes): `cfg.Storage.Attachment.MaxUploadSizeBytes`
  - HTTP 레이어는 multipart body 크기를 먼저 제한하고, service 레이어는 파일 스트림 크기를 다시 검증합니다.
- attachment 이미지 최적화: `cfg.Storage.Attachment.ImageOptimization.Enabled`, `cfg.Storage.Attachment.ImageOptimization.JPEGQuality`
- background jobs on/off: `cfg.Jobs.Enabled`
- attachment cleanup 주기/유예/배치 크기: `cfg.Jobs.AttachmentCleanup.*`
  - 기본 유예는 `600`초이며, orphan와 `pending_delete` attachment 모두 같은 cleanup 주기를 사용합니다.
- guest cleanup 주기/유예/배치 크기: `cfg.Jobs.GuestCleanup.*`
  - `pending`/`expired` guest는 `pendingGracePeriodSeconds` 기준으로 정리합니다.
  - 세션 없음 + 작성물 없음 상태의 `active guest`는 `activeUnusedGracePeriodSeconds` 기준으로 정리합니다.
- email verification cleanup 주기/유예/배치 크기: `cfg.Jobs.EmailVerificationCleanup.*`
  - `ConsumedAt <= now - gracePeriod` 또는 `ExpiresAt <= now - gracePeriod` 인 token을 background job이 삭제합니다.
- password reset cleanup 주기/유예/배치 크기: `cfg.Jobs.PasswordResetCleanup.*`
  - `ConsumedAt <= now - gracePeriod` 또는 `ExpiresAt <= now - gracePeriod` 인 token을 background job이 삭제합니다.

## 운영 메모

- 커밋된 `config.yml`은 샘플로 취급합니다.
- 실제 실행 전에는 `delivery.http.auth.secret`를 반드시 실값으로 넣어야 합니다.
- 운영 환경에서는 예측 가능한 문자열 대신 충분히 긴 랜덤 시크릿(최소 32자 이상)을 사용합니다.
- bootstrap admin은 샘플 `config.yml`에 포함되어 있다. 운영 환경에서는 호스트 config로 덮어쓰거나 별도 시드 전략을 사용한다.
- 관리용 점검 스크립트는 `scripts/check-bootstrap-admin.sh`를 사용한다.
