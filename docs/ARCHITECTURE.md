# 아키텍처

## 핵심 원칙

- Layered Architecture
- Port & Adapter (Hexagonal)
- Domain 중심 설계
- interface/implementation 분리

## 레이어 구조

```mermaid
flowchart LR
    HTTP["Delivery (HTTP / middleware / response)"]
    UC["UseCase Port"]
    APP["Application Service / Policy / Cache Rule / Read Assembly"]
    PORT["Repository / External Port"]
    INFRA["Infrastructure Adapter (inmemory / auth / cache / storage)"]
    DOMAIN["Domain Entity"]

    HTTP --> UC
    UC --> APP
    APP --> DOMAIN
    APP --> PORT
    PORT --> INFRA
```

- `delivery`
  - HTTP 파싱, 인증 미들웨어 연결, status/header/response 직렬화만 담당한다.
  - request body와 multipart 같은 transport 경계 제한도 여기서 먼저 차단한다.
- `application`
  - 유스케이스 orchestration, 권한 판정, 캐시 정책, tx 경계, read model 조립을 담당한다.
  - 로깅이 필요하면 전역 구현에 직접 의존하지 않고 application port를 통해 호출한다.
- `domain`
  - 엔티티 상태와 도메인 규칙을 가진다.
- `infrastructure`
  - 저장소, 캐시, 토큰, 파일 저장소 같은 외부 구현체를 제공한다.

읽기 경로에서도 동일한 경계를 유지한다.

- Delivery는 쿼리 파라미터/헤더를 해석하고 UseCase만 호출한다.
- Application은 필요한 read assembly를 수행하되, repository를 반복 호출하는 N+1 패턴은 가능한 포트 확장이나 query helper로 흡수한다.
- read path가 커지는 경우 service 안에 계속 누적하지 않고, `postDetailQuery` 같은 read-side query component로 분리한다.
- Infrastructure는 batched read 같은 조회 최적화를 구현 세부로 숨긴다.

## 요청 흐름

`HTTP Delivery -> UseCase Port -> Service -> Repository Port -> InMemory Adapter`

Composition root는 `cmd/main.go` 에 두고, wiring 단계에서만 concrete 구현체를 조립합니다.

## 사용자 식별자 정책

- 내부 PK/FK는 `int64`를 유지한다.
- 외부에 노출하는 사용자 식별자는 `User.UUID`를 사용한다.
- 게시글/댓글/리액션 응답은 내부 `author_id`/`user_id` 대신 `author_uuid`/`user_uuid`를 노출한다.
- soft delete 후에도 `uuid`는 유지되며, `name` 같은 식별 정보만 익명화한다.

## 인증/인가 흐름

- 인증
  - `gin` middleware가 `Authorization` 헤더에서 토큰 추출
  - `SessionUseCase`가 JWT 검증 + `SessionRepository` 세션 확인 수행
  - 검증 성공 후 `context.user_id` 주입
- 인가
  - Service 레이어에서 주입된 `AuthorizationPolicy`로 권한 판정
  - 기본 정책: `AdminOnly`, `OwnerOrAdmin`
  - suspension 운영 API의 외부 대상 식별자는 `user_uuid`를 사용한다.

## 세션 유효성 흐름

- login: `SessionUseCase`가 사용자 인증 후 토큰 발급 + `SessionRepository` 저장
- 현재 In-Memory 구현은 cache 기반 `SessionRepository`를 사용한다.
- 세션 키는 `user_id + token` 기준으로 관리해 사용자 단위 무효화가 가능하도록 유지
- protected route: middleware가 `SessionUseCase`로 세션 검증
- logout: `SessionUseCase`가 해당 토큰 세션만 삭제
- delete me: `AccountUseCase`가 계정 삭제 후 해당 사용자의 세션 전체 무효화를 시도
- 현재 계정 삭제 후 세션 정리는 best effort 정책이며, 실패 시 구조화 로그만 남기고 계정 삭제 성공을 유지

## 비밀번호 처리

- 비밀번호 해시/비교는 `port.PasswordHasher`로 추상화한다.
- `UserService`는 평문을 저장하지 않고, signup 시 해시 후 저장한다.
- 인증/탈퇴 시에는 저장된 해시와 입력 비밀번호를 비교한다.
- 탈퇴는 현재 soft delete + 익명화 정책을 사용한다.
- 탈퇴 시 사용자명, 이메일, 비밀번호 같은 식별 정보는 더 이상 로그인/식별에 사용할 수 없게 비식별화한다.
- `UserRepository.SelectUserByID`, `SelectUserByUsername`는 soft deleted 사용자를 기본 조회에서 제외한다.
- 작성물/리액션 같은 기존 데이터 참조를 위해 `SelectUserByIDIncludingDeleted`를 별도로 둔다.

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
- 4xx/5xx 모두 composition root에서 주입된 로거를 통해 구조화 로그를 남긴다.
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

주의: 캐시 정책(TTL, 키 규칙, 무효화 범위)은 Delivery가 아니라 Application 레이어에서 관리하는 것을 기본 원칙으로 한다.

## 이벤트 경계 (EDA 1차)

- Application Port
  - `DomainEvent`: `EventName()`, `OccurredAt()`
  - `EventPublisher`: `Publish(events ...DomainEvent)`
  - `EventBus`: `Subscribe`, `Publish`
- Event Type (최소 세트)
  - `BoardChanged`
  - `PostChanged`
  - `CommentChanged`
  - `ReactionChanged`
  - `AttachmentChanged`
- 디스패치
  - 트랜잭션 성공 후 서비스가 이벤트를 발행한다.
  - in-process bus가 비동기로 핸들러를 호출한다.
  - 핸들러 panic/error는 복구 후 warn 로그로만 남긴다.

## 쓰기 이후 캐시 무효화 규칙

- repository write 성공이 시스템의 기준 성공이다.
- write 이후 조회 캐시 무효화는 이벤트 소비자에서 best effort로 처리한다.
- 이벤트 처리/캐시 무효화가 실패해도 write API는 성공을 유지하고, 실패는 구조화 로그로만 남긴다.
- 조회 캐시가 stale 상태로 잠시 남을 수 있으므로 TTL과 후속 write 이벤트로 수렴한다.

## 캐시 구성 기준

- `internal/application/port/cache.go`
  - 캐시 포트(interface) 정의
- `internal/application/cache/policy.go`
  - 서비스에서 사용하는 캐시 TTL 정책 모델
- `internal/application/cache/key/keys.go`
  - 캐시 키 생성 규칙
- `internal/application/cache/testutil/spy_cache.go`
  - 서비스 정책 회귀 검증용 테스트 유틸
- `internal/application/port/event.go`
  - 도메인 이벤트/버스 포트 정의
- `internal/application/event`
  - 이벤트 타입 + 캐시 무효화 핸들러
- `internal/infrastructure/event/inprocess`
  - 비동기 in-process event bus
- `internal/infrastructure/cache/inmemory`
  - 실사용 캐시 어댑터
- `internal/infrastructure/cache/noop`
  - 테스트/폴백용 noop 어댑터

포트는 `internal/application/port` 아래에 두고 구현체는 `infrastructure`에 둔다.

## 페이징/캐시 정책 위치

- 목록 조회는 커서 기반(`limit`, `last_id`)을 기본으로 한다.
- 조회 핸들러가 아닌 서비스가 아래를 수행한다.
  - 조회 캐시 적중/미스 처리(`GetOrSetWithTTL`)
  - 쓰기 이후 도메인 이벤트 발행
  - 부모 리소스의 공개 상태 검증

## 삭제 일관성 규칙

- 삭제된 게시글은 공개 댓글/리액션 조회 대상이 아니다.
- 게시글 삭제 시 하위 댓글은 soft delete 처리한다.
- 댓글 공개 조회는 기본적으로 `active`만 노출하되, 활성 reply가 남아 있는 삭제된 부모 댓글은 tombstone으로 함께 노출한다.
- 게시글 삭제 시 첨부는 orphan 처리해 cleanup job이 수거하도록 한다.
- attachment 삭제는 즉시 hard delete 하지 않고 `pending_delete` 로 숨긴 뒤, cleanup job이 실제 파일 삭제와 메타데이터 hard delete를 재시도한다.
- 게시글/댓글 삭제 시 하위 리액션 메타데이터도 함께 정리한다.
- 게시판 삭제는 비어 있는 경우에만 허용한다.

## 첨부 작성 플로우

- attachment embed가 필요한 글 작성은 `draft-first` workflow를 기본으로 한다.
- 흐름은 `draft 생성 -> attachment 업로드 -> 본문 수정(attachment://{id} 참조) -> publish` 이다.
- 공개 글/임시글 생성 시점에는 본문에 `attachment://{id}` 참조를 허용하지 않는다.
- attachment는 기존 post ID에 귀속되는 메타데이터이므로, post 생성 전 임시 업로드 토큰 모델은 현재 채택하지 않는다.
- 업로드 크기 제한은 delivery에서 multipart 파싱 전에 1차 차단하고, attachment service가 파일 스트림 크기를 다시 검증한다.

## In-Memory 저장소 동시성

- `internal/infrastructure/persistence/inmemory/*Repository`는 `sync.RWMutex`로 보호한다.
- 목적
  - API 통합 테스트/실행 중 동시 접근 시 데이터 경합 방지
  - 읽기(`RLock`)와 쓰기(`Lock`) 구분으로 기본 성능/안전 확보
- 조회 결과는 저장소 내부 객체를 직접 노출하지 않도록 clone을 반환한다.
- 서비스는 저장소에서 읽은 엔티티를 직접 공유 상태로 간주하지 않고, 수정 시 copy-on-write 후 `Update`를 호출한다.
- 이 규칙은 `UserRepository`, `BoardRepository`, `PostRepository`, `TagRepository`, `PostTagRepository`, `CommentRepository`, `ReactionRepository`, `AttachmentRepository` 전체에 적용한다.
- 단, 동시성 보호만으로 충분하지 않은 저장소는 포트 계약 자체에 원자적 의미를 둔다.
  - 예: `ReactionRepository`는 `(user_id, target_id, target_type)` 유니크를 저장소 레벨에서 보장
  - 예: `UserRepository`는 `username`, `uuid` 유니크를 저장소 레벨에서 보장

## 저장소 계약 테스트

- 단순 CRUD를 넘는 의미 계약이 있는 포트는 공통 contract test로 검증한다.
- 위치
  - `internal/application/porttest`
- 목적
  - 새 구현체가 들어와도 동일한 의미론을 재사용 가능한 테스트로 검증
  - 인터페이스만으로 강제할 수 없는 유니크 제약, cursor 규약, no-op 규약을 테스트로 고정
- 현재 적용 대상
  - `AttachmentRepository`: orphan/pending-delete cleanup 후보 선별 규약
  - `CommentRepository`: tombstone 노출용 deleted 포함 조회 규약
  - `ReactionRepository`: user-target 유니크, 원자적 set/delete
  - `UserRepository`: username/uuid 유니크, soft delete 조회 규약
  - `BoardRepository`: cursor pagination 정렬/절단 규약
  - `PostRepository`: 공개 조회/soft delete/board 존재 검사용 의미 규약

## 다중 저장소 쓰기 단위

- 여러 도메인을 함께 갱신하는 쓰기 유스케이스는 `UnitOfWork` 포트를 통해 하나의 작업 단위로 실행한다.
- 목적은 RDB transaction 자체를 노출하는 것이 아니라, 애플리케이션 레벨에서 명시적 tx 경계를 분리하는 것이다.
- 포트는 어댑터 독립적이며, 이후 RDB 어댑터는 실제 DB transaction으로, in-memory 어댑터는 동일 의미의 tx-bound repository + rollback 메커니즘으로 매핑한다.
- 현재 `Tag` 도메인 도입으로 `Post`, `Tag`, `PostTag`가 함께 갱신되므로 `PostService` create/update/delete 경로에 이 원칙을 적용한다.
- `CommentService.DeleteComment`처럼 댓글/리액션을 함께 갱신하는 경로도 동일 원칙을 적용한다.
- in-memory 구현은 tx-bound repository 접근, snapshot rollback, tx 수행 중 외부 repository 접근 차단을 제공한다.
- write 서비스의 기본 규칙은 `조회 -> 검증 -> 갱신`을 같은 tx 안에서 수행하는 것이다.
- 캐시 삭제 호출은 tx 밖에서 best effort로 처리하되, 무효화 대상 식별자 수집은 tx 안에서 마친다.

## Tag 기반 Post 조회 책임

- `published posts by tag`는 입력이 tag여도 반환 aggregate와 공개 정책이 `Post`에 속하므로 `PostRepository` 책임으로 둔다.
- 현재는 `PostRepository.SelectPublishedPostsByTagName(...)`가 `post.id` 커서와 `status=published` 기준으로 조회를 수행한다.
- `PostTag`는 relation 생명주기(`active/deleted`)만 책임지고, 공개 노출 규칙은 가지지 않는다.

## 구성 루트 (Composition Root)

- 파일: `cmd/main.go`
- 역할
  - config 로딩
  - repository/service/policy/auth/cache 조립
  - HTTP 서버 시작
  - optional bootstrap admin 생성(`admin.bootstrap.enabled=true`일 때만)

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
      account_usecase.go
      credential_verifier.go
      password_hasher.go
      session_repository.go
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
      cache_errors.go
      user_reference.go
      *.go

  domain/
    entity/
      *.go

  infrastructure/
    auth/
      BcryptPasswordHasher.go
      CacheSessionRepository.go
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

## 사용자 저장소 규칙

- `UserRepository`는 공개 조회와 내부 참조 조회를 구분한다.
  - `SelectUserByUsername`, `SelectUserByID`: soft deleted 사용자 제외
  - `SelectUserByIDIncludingDeleted`: 작성물/리액션 응답용 내부 참조
- `Save`, `Update`는 `username`과 `uuid` 유니크를 보장한다.
- 이 구조는 soft delete 후에도 공개 로그인/조회와 내부 참조 복원을 분리하는 목적을 가진다.
