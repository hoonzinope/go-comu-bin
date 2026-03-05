# 아키텍처

## 핵심 원칙

- Layered Architecture
- Port & Adapter (Hexagonal)
- Domain 중심 설계
- interface/implementation 분리

## 요청 흐름

`HTTP Delivery -> UseCase Port -> Service -> Repository Port -> InMemory Adapter`

## 인증/인가 흐름

- 인증
  - `gin` middleware가 `Authorization` 헤더에서 토큰 추출
  - JWT 검증 후 `context.user_id` 주입
  - token cache 조회로 유효 세션 확인
- 인가
  - Service 레이어에서 `AuthorizationPolicy`로 권한 판정
  - 기본 정책: `AdminOnly`, `OwnerOrAdmin`

## 세션 유효성 흐름

- login: 토큰 발급 후 cache 저장
- protected route: JWT 검증 + cache 확인
- logout: cache에서 토큰 삭제(무효화)

## 캐시 포트 확장

- `application.Cache`는 인증 캐시와 조회 캐시를 단일 포트로 관리
- 기본 연산
  - `Get`, `Set`, `SetWithTTL`, `Delete`
- 조회 캐시 확장 연산
  - `DeleteByPrefix(prefix)`: 쓰기 이벤트 후 관련 캐시 일괄 무효화
  - `GetOrSetWithTTL(key, ttl, loader)`: 캐시 미스 시 로더 실행 후 저장
- In-Memory 구현 세부
  - 만료(`TTL`) 지원
  - `singleflight` 기반 동시성 중복 로더 호출 방지

주의: 캐시 정책(TTL, 키 규칙, 무효화 범위)은 Delivery가 아니라 Service 레이어에서 관리하는 것을 기본 원칙으로 한다.

## 캐시 구성 기준

- `internal/application/cache.go`
  - 캐시 포트(interface) 정의
- `internal/application/cache/policy.go`
  - 서비스에서 사용하는 캐시 TTL 정책 모델
- `internal/application/cache/key/keys.go`
  - 캐시 키 생성 규칙
- `internal/application/cache/testutil/spy_cache.go`
  - 서비스 정책 회귀 검증용 테스트 유틸
- `internal/infrastructure/cache/inmemory`
  - 실사용 캐시 어댑터
- `internal/infrastructure/cache/noop`
  - 테스트/폴백용 noop 어댑터

`cache/interface` 같은 별도 폴더 분리 대신, 포트는 `application`에 두고 구현체는 `infrastructure`에 둔다.

## 페이징/캐시 정책 위치

- 목록 조회는 커서 기반(`limit`, `last_id`)을 기본으로 한다.
- 조회 핸들러가 아닌 서비스가 아래를 수행한다.
  - 조회 캐시 적중/미스 처리(`GetOrSetWithTTL`)
  - 쓰기 이후 관련 캐시 무효화(`Delete`, `DeleteByPrefix`)

## In-Memory 저장소 동시성

- `internal/infrastructure/persistence/inmemory/*Repository`는 `sync.RWMutex`로 보호한다.
- 목적
  - API 통합 테스트/실행 중 동시 접근 시 데이터 경합 방지
  - 읽기(`RLock`)와 쓰기(`Lock`) 구분으로 기본 성능/안전 확보

## 구성 루트 (Composition Root)

- 파일: `cmd/main.go`
- 역할
  - config 로딩
  - repository/usecase/auth/cache 조립
  - HTTP 서버 시작
  - admin 계정 시드(`admin/admin`)

## 디렉토리 구조

```txt
cmd/
  main.go

internal/
  delivery/
    http.go
    middleware/
      authMiddleware.go
    response/
      types.go
      mapper.go

  application/
    authentication.go
    cache.go
    cache/
      policy.go
      key/
        keys.go
      testutil/
        spy_cache.go
    useCase.go
    repository.go
    policy/
      authorization_policy.go
      role_authorization_policy.go
    service/
      *.go

  domain/
    entity/
      *.go
    dto/
      *.go

  infrastructure/
    auth/
      JwtTokenProvider.go
    cache/
      inmemory/
        in_memory_cache.go
      noop/
        noop_cache.go
    persistence/
      inmemory/
        *.go

  customError/
    customError.go
```

## 모델 분리 원칙

- `domain/entity`: 비즈니스 모델(직렬화 관심사 없음)
- `domain/dto`: 유스케이스 반환 모델
- `delivery/response`: HTTP 응답 스키마(JSON 태그 정의)

도메인 엔티티에는 `json` 태그를 두지 않고, 전달 계층에서 응답 모델로 매핑합니다.
