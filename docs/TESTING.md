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
go test ./internal/application/service -v
go test ./internal/delivery -v
go test ./internal/infrastructure/persistence/inmemory -v
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
	postID := seedPost(repositories.post, ownerID, 1, "title", "content")
	svc := NewPostService(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.comment,
		repositories.reaction,
		newTestCache(),
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)

	err := svc.UpdatePost(postID, otherID, "new-title", "new-content")

	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}
```

## 테스트 구성

- Unit
  - `internal/domain/entity`
  - `internal/infrastructure/cache/inmemory`
  - `internal/infrastructure/auth`
  - `internal/infrastructure/persistence/inmemory`
  - `internal/application/porttest`
  - `internal/application/service`
  - `internal/delivery`
- Integration
  - `internal/integration`

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
