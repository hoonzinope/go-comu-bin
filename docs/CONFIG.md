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
  local:
    rootDir: "./data/uploads"

delivery:
  http:
    port: 18577
    auth:
      secret: "commu-bin-secret-key"
```

## 검증 규칙

- `delivery.http.port`: `1..65535`
- `delivery.http.auth.secret`: 필수(빈 값 불가)
- `cache.listTTLSeconds`: `> 0`
- `cache.detailTTLSeconds`: `> 0`
- `storage.local.rootDir`: 필수(빈 값 불가)
- 알 수 없는 키는 실패 처리 (`UnmarshalExact`)
  - 예: `delivery.http.prt` 오타는 서버 시작 실패

## 사용 위치

- 포트: `cmd/main.go` -> `cfg.Delivery.HTTP.Port`
- JWT 시크릿: `cmd/main.go` -> `cfg.Delivery.HTTP.Auth.Secret`
- 캐시 TTL 정책: `cmd/main.go` -> `cfg.Cache.ListTTLSeconds`, `cfg.Cache.DetailTTLSeconds`
- 로컬 업로드 루트: `cfg.Storage.Local.RootDir`
