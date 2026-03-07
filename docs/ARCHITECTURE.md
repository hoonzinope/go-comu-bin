# 아키텍처

## 핵심 원칙

- Layered Architecture
- Port & Adapter (Hexagonal)
- Domain 중심 설계
- interface/implementation 분리

## 요청 흐름

`HTTP Delivery -> UseCase Port -> Service -> Repository Port -> InMemory Adapter`

Composition root는 `cmd/main.go` 에 두고, wiring 단계에서만 concrete 구현체를 조립합니다.

## 인증/인가 흐름

- 인증
  - `gin` middleware가 `Authorization` 헤더에서 토큰 추출
  - `SessionUseCase`가 JWT 검증 + 세션 cache 확인 수행
  - 검증 성공 후 `context.user_id` 주입
- 인가
  - Service 레이어에서 주입된 `AuthorizationPolicy`로 권한 판정
  - 기본 정책: `AdminOnly`, `OwnerOrAdmin`

## 세션 유효성 흐름

- login: `SessionUseCase`가 사용자 인증 후 토큰 발급 + cache 저장
- protected route: middleware가 `SessionUseCase`로 세션 검증
- logout: `SessionUseCase`가 cache에서 토큰 삭제(무효화)

## 에러 처리 원칙

- 서비스는 원인 분류를 잃지 않도록 에러를 래핑한다.
  - 저장소 실패: `customError.ErrRepositoryFailure`
  - 캐시 실패: `customError.ErrCacheFailure`
  - 토큰 발급 실패: `customError.ErrTokenFailure`
- 외부 계약으로 노출되는 에러는 공개 sentinel로 수렴한다.
  - 예: `ErrUserNotFound`, `ErrForbidden`, `ErrInvalidToken`
  - HTTP 계층은 `customError.Public(err)`로 응답 메시지를 정규화한다.
- 내부 원인 문자열은 응답에 직접 노출하지 않는다.
  - 운영 추적은 구조화 로그에서 수행한다.

## HTTP 예외 처리/로깅

- `delivery.writeUseCaseError`가 공개 에러 -> HTTP status 매핑의 단일 진입점이다.
- 4xx/5xx 모두 `log/slog` 기반 구조화 로그를 남긴다.
  - 공통 필드: method, path, request_uri, user_id(가능 시), status, public_error
- 5xx는 상세 원인을 로그로 남기되, 응답에는 `internal server error`만 노출한다.

## 캐시 포트 확장

- `port.Cache`는 인증 캐시와 조회 캐시를 단일 포트로 관리
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

- `internal/application/port/cache.go`
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

포트는 `internal/application/port` 아래에 두고 구현체는 `infrastructure`에 둔다.

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
- 단, 동시성 보호만으로 충분하지 않은 저장소는 포트 계약 자체에 원자적 의미를 둔다.
  - 예: `ReactionRepository`는 `(user_id, target_id, target_type)` 유니크를 저장소 레벨에서 보장
  - 예: `UserRepository`는 `username` 유니크를 저장소 레벨에서 보장

## 저장소 계약 테스트

- 단순 CRUD를 넘는 의미 계약이 있는 포트는 공통 contract test로 검증한다.
- 위치
  - `internal/application/porttest`
- 목적
  - 새 구현체가 들어와도 동일한 의미론을 재사용 가능한 테스트로 검증
  - 인터페이스만으로 강제할 수 없는 유니크 제약, cursor 규약, no-op 규약을 테스트로 고정
- 현재 적용 대상
  - `ReactionRepository`: user-target 유니크, 원자적 set/delete
  - `UserRepository`: username 유니크
  - `BoardRepository`: cursor pagination 정렬/절단 규약

## 구성 루트 (Composition Root)

- 파일: `cmd/main.go`
- 역할
  - config 로딩
  - repository/service/policy/auth/cache 조립
  - HTTP 서버 시작
  - admin 계정 시드(`admin/admin`)

## 조립 원칙

- 애플리케이션 포트는 `internal/application/port` 아래에 둔다.
- 서비스는 필요한 repository port만 직접 받는다.
- wiring 편의를 위해 `delivery.NewHTTPServer` 는 `HTTPDependencies` struct를 사용한다.
- aggregate struct로 서비스 경계를 숨기지 않고, 조립 단계에서만 의존성을 묶는다.

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
    mapper/
      dto_mapper.go
    model/
      *.go
    port/
      cache.go
      credential_verifier.go
      token_provider.go
      session_usecase.go
      user_usecase.go
      board_usecase.go
      post_usecase.go
      comment_usecase.go
      reaction_usecase.go
      user_repository.go
      board_repository.go
      post_repository.go
      comment_repository.go
      reaction_repository.go
    cache/
      policy.go
      key/
        keys.go
      testutil/
        spy_cache.go
    policy/
      authorization_policy.go
      role_authorization_policy.go
    service/
      *.go

  domain/
    entity/
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
- `application/model`: 유스케이스 반환 모델(entity 비노출 projection)
- `delivery/response`: HTTP 응답 스키마(JSON 태그 정의)

도메인 엔티티에는 `json` 태그를 두지 않고, 서비스가 entity를 application model로 변환한 뒤 전달 계층에서 HTTP 응답 모델로 다시 매핑합니다.

## 리액션 타입 규칙

- 리액션 대상과 종류는 raw string 대신 domain type으로 관리한다.
  - `entity.ReactionTargetType`
  - `entity.ReactionType`
- delivery는 HTTP 문자열 입력을 파싱한 뒤 typed value로 service에 전달한다.
- 이로 인해 `"post"`, `"comment"`, `"like"` 같은 프로토콜 문자열이 서비스/저장소 경계 전반에 흩어지는 것을 줄인다.

## 리액션 저장소 규칙

- `ReactionRepository`는 일반 CRUD가 아니라 user-target 중심 전용 연산을 제공한다.
  - `SetUserTargetReaction`
  - `DeleteUserTargetReaction`
  - `GetUserTargetReaction`
  - `GetByTarget`
- 서비스는 더 이상 `조회 -> 판단 -> 저장`을 조합하지 않고, 저장소의 원자적 계약을 사용한다.
- 이 구조는 In-Memory 단계에서도 중복 리액션 race를 막고, SQLite 단계에서는 `UNIQUE(user_id, target_id, target_type)`와 자연스럽게 대응된다.
