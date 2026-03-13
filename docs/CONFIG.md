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

## 예시

```yaml
cache:
  listTTLSeconds: 30
  detailTTLSeconds: 30

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
    auth:
      secret: "replace-with-real-secret"

event:
  outbox:
    workerCount: 1
    batchSize: 100
    pollIntervalMillis: 100
    maxAttempts: 5
    baseBackoffMillis: 200

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
```

## 검증 규칙

- `delivery.http.port`: `1..65535`
- `delivery.http.maxJSONBodyBytes`: `> 0`
- `delivery.http.auth.secret`: 필수(빈 값 불가)
- `delivery.http.auth.secret`: placeholder 값 금지 (`commu-bin-secret-key`)
- `delivery.http.auth.secret`: 최소 길이 `16`자 이상
- `event.outbox.workerCount`: `> 0`
- `event.outbox.batchSize`: `> 0`
- `event.outbox.pollIntervalMillis`: `> 0`
- `event.outbox.maxAttempts`: `> 0`
- `event.outbox.baseBackoffMillis`: `> 0`
- `admin.bootstrap.enabled`: 기본 `false`
- `admin.bootstrap.username`: bootstrap enabled일 때 필수
- `admin.bootstrap.password`: bootstrap enabled일 때 필수
- `admin.bootstrap.password`: placeholder 값 금지 (`admin`)
- `cache.listTTLSeconds`: `> 0`
- `cache.detailTTLSeconds`: `> 0`
- `storage.local.rootDir`: 필수(빈 값 불가)
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
- 알 수 없는 키는 실패 처리 (`UnmarshalExact`)
  - 예: `delivery.http.prt` 오타는 서버 시작 실패

## 사용 위치

- 포트: `cmd/main.go` -> `cfg.Delivery.HTTP.Port`
- JSON body 최대 크기(bytes): `cmd/main.go` -> `cfg.Delivery.HTTP.MaxJSONBodyBytes`
  - JSON API 요청 바디가 이 값을 초과하면 `400 Bad Request (request body too large)`를 반환합니다.
- JWT 시크릿: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.Secret`
- outbox relay 워커 수: `cmd/main.go` -> `cfg.Event.Outbox.WorkerCount`
- outbox relay 배치 크기: `cmd/main.go` -> `cfg.Event.Outbox.BatchSize`
- outbox relay polling 주기(ms): `cmd/main.go` -> `cfg.Event.Outbox.PollIntervalMillis`
- outbox retry 최대 횟수: `cmd/main.go` -> `cfg.Event.Outbox.MaxAttempts`
- outbox retry base backoff(ms): `cmd/main.go` -> `cfg.Event.Outbox.BaseBackoffMillis`
  - 전달은 at-least-once이며, 실패 이벤트는 backoff 재시도 후 `dead` 상태로 남깁니다.
- bootstrap admin: `cmd/main.go` -> `cfg.Admin.Bootstrap.*`
- 캐시 TTL 정책: `cmd/main.go` -> `cfg.Cache.ListTTLSeconds`, `cfg.Cache.DetailTTLSeconds`
- 로컬 업로드 루트: `cfg.Storage.Local.RootDir`
- 파일 저장 provider: `cfg.Storage.Provider`
- object storage endpoint/bucket: `cfg.Storage.Object.Endpoint`, `cfg.Storage.Object.Bucket`
- attachment 최대 업로드 크기(bytes): `cfg.Storage.Attachment.MaxUploadSizeBytes`
  - HTTP 레이어는 multipart body 크기를 먼저 제한하고, service 레이어는 파일 스트림 크기를 다시 검증합니다.
- attachment 이미지 최적화: `cfg.Storage.Attachment.ImageOptimization.Enabled`, `cfg.Storage.Attachment.ImageOptimization.JPEGQuality`
- background jobs on/off: `cfg.Jobs.Enabled`
- attachment cleanup 주기/유예/배치 크기: `cfg.Jobs.AttachmentCleanup.*`
  - 기본 유예는 `600`초이며, orphan와 `pending_delete` attachment 모두 같은 cleanup 주기를 사용합니다.

## 운영 메모

- 커밋된 `config.yml`은 샘플로 취급합니다.
- 실제 실행 전에는 `delivery.http.auth.secret`를 반드시 실값으로 넣어야 합니다.
- 운영 환경에서는 예측 가능한 문자열 대신 충분히 긴 랜덤 시크릿(최소 16자, 권장 32자+)을 사용합니다.
- bootstrap admin이 필요할 때만 `admin.bootstrap.enabled=true`로 켜고, 일회성 강한 비밀번호를 설정한 뒤 다시 끄는 것을 기본으로 합니다.
