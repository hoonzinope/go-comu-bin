# 테스트 가이드

## 실행

```bash
go test ./...
```

환경에 따라 캐시 경로 제한이 있으면:

```bash
GOCACHE=/tmp/go-comu-bin-gocache go test ./...
```

패키지별 실행:

```bash
go test ./internal/application/service/... -v
go test ./internal/delivery -v
go test ./internal/delivery/web -v
go test ./internal/infrastructure/persistence/inmemory -v
go test ./internal/infrastructure/cache/ristretto -v
go test ./internal/infrastructure/cache/inmemory -v
go test ./internal/domain/entity -v
go test ./internal/integration -v
```

## 테스트 코드 스타일

참고: [우리가 테스트를 하는 이유. 근데 이제 Golang을 곁들인](https://blog.banksalad.com/tech/why-we-do-test-by-golang/)

### 핵심 원칙

- 테스트는 동작 문서처럼 읽혀야 함
- 준비 단계 실패와 검증 실패를 분리
- 반복 패턴은 헬퍼로 추출하되 의도를 숨기지 않음

### Assertion 규칙

- 기본: `github.com/stretchr/testify`
- 전제/에러 확인: `require`
- 결과 검증: `assert`

### 이름/구조 규칙

- `Test<대상>_<시나리오>` 권장
- 복수 시나리오는 `t.Run("given_..._when_..._then_...")`
- 하나의 테스트는 하나의 행동/정책 검증

### 작성 패턴

- `given -> when -> then`

```go
func TestPostService_UpdatePost_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewPostService(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		newTestCache(),
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)

	err := svc.UpdatePost(context.Background(), mustPostUUID(t, repositories.post, postID), otherID, "new-title", "new-content", nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}
```

## 테스트 구성

- Unit
  - `internal/infrastructure/cache/ristretto`
  - `internal/domain/entity`
  - `internal/infrastructure/cache/inmemory`
  - `internal/infrastructure/auth`
  - `internal/infrastructure/persistence/inmemory`
  - `internal/application/porttest`
  - `internal/application/service/...`
  - `internal/delivery`
  - `internal/delivery/web`
  - `cmd`의 logger wiring 및 panic recovery
  - `internal/infrastructure/job/inprocess`의 panic recovery
  - `internal/infrastructure/event/outbox`의 panic recovery
- Integration
  - `internal/integration`

## Browser E2E / Visual Regression

- Playwright 설정: `playwright.config.ts`
- 테스트 위치: `tests/e2e/*.spec.ts`
- 전체 실행: `npm run test:e2e`
- Chromium 시나리오: `npm run test:e2e:chromium`
- 시각 회귀: `npm run test:e2e:visual`
- 시각 기준 갱신: `npm run test:e2e:visual:update`

Visual snapshot baselines are stored under `tests/e2e/visual.spec.ts-snapshots/`.
Intentional UI changes가 아니면 visual baselines는 갱신하지 않는다.

## 로깅/예외 테스트 기준

- `stdout + lumberjack` logger wiring은 `cmd` 테스트에서 검증한다.
- HTTP panic recovery는 500 응답과 구조화 로그 모두를 확인한다.
- background job / outbox relay panic은 worker를 죽이지 않고 다음 tick/loop로 이어지는지 확인한다.

## 저장소 Contract Test 기준

- 인터페이스 구현 여부만으로는 의미 계약을 보장할 수 없으므로, 저장소별 공통 contract test를 둔다.
- 위치
  - `internal/application/porttest`
- 구현체 테스트는 각 저장소 테스트 파일에서 contract runner를 호출한다.
  - 예: `UserRepository`
  - 예: `BoardRepository`
  - 예: `ReactionRepository`
- contract test가 필요한 경우
  - 유니크 제약이 저장소 의미에 포함될 때
  - cursor pagination/정렬/no-op 같은 규약을 서비스가 기대할 때
  - 동시성 상황에서도 동일 의미를 유지해야 할 때

## 캐시 정책 회귀 테스트 기준

- 별도 `cache_behavior_test` 파일로 분리하지 않고, 각 서비스 테스트 파일에 정책 검증을 포함한다.
  - 예: 게시글 상세 캐시 hit/miss, 쓰기 후 목록/상세 무효화
- 공통 테스트 더블은 `internal/application/cache/testutil`에 둔다.
  - `SpyCache`: `GetOrSetWithTTL` 로더 호출 횟수, `DeleteByPrefix` 무효화 여부 검증
- 권한 정책이 필요한 서비스 테스트는 `newTestAuthorizationPolicy()`로 명시적으로 주입한다.
