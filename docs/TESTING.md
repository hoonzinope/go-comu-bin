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
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	postID := seedPost(repository, ownerID, 1, "title", "content")
	svc := NewPostService(repository)

	err := svc.UpdatePost(postID, otherID, "new-title", "new-content")

	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}
```

## 테스트 구성

- Unit
  - `internal/domain/entity`
  - `internal/infrastructure/persistence/inmemory`
  - `internal/application/service`
  - `internal/delivery`
- Integration
  - `internal/integration`
