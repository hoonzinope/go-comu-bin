# 설정 (Config)

## 로딩 위치

서버 시작 시 아래 순서로 설정 파일을 탐색합니다.

- `./config.yml`
- `./config/config.yml`

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
    auth:
      secret: "commu-bin-secret-key"

jobs:
  enabled: true
  orphanAttachmentCleanup:
    enabled: true
    intervalSeconds: 600
    gracePeriodSeconds: 600
    batchSize: 50
```

## 검증 규칙

- `delivery.http.port`: `1..65535`
- `delivery.http.auth.secret`: 필수(빈 값 불가)
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
- `jobs.orphanAttachmentCleanup.intervalSeconds`: `> 0`
- `jobs.orphanAttachmentCleanup.gracePeriodSeconds`: `> 0`
- `jobs.orphanAttachmentCleanup.batchSize`: `> 0`
- 알 수 없는 키는 실패 처리 (`UnmarshalExact`)
  - 예: `delivery.http.prt` 오타는 서버 시작 실패

## 사용 위치

- 포트: `cmd/main.go` -> `cfg.Delivery.HTTP.Port`
- JWT 시크릿: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.Secret`
- 캐시 TTL 정책: `cmd/main.go` -> `cfg.Cache.ListTTLSeconds`, `cfg.Cache.DetailTTLSeconds`
- 로컬 업로드 루트: `cfg.Storage.Local.RootDir`
- 파일 저장 provider: `cfg.Storage.Provider`
- object storage endpoint/bucket: `cfg.Storage.Object.Endpoint`, `cfg.Storage.Object.Bucket`
- attachment 최대 업로드 크기(bytes): `cfg.Storage.Attachment.MaxUploadSizeBytes`
- attachment 이미지 최적화: `cfg.Storage.Attachment.ImageOptimization.Enabled`, `cfg.Storage.Attachment.ImageOptimization.JPEGQuality`
- background jobs on/off: `cfg.Jobs.Enabled`
- orphan attachment cleanup 주기/유예/배치 크기: `cfg.Jobs.OrphanAttachmentCleanup.*`
  - 기본 유예는 `600`초이며, orphan와 `pending_delete` attachment 모두 같은 cleanup 주기를 사용합니다.
