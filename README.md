# go-comu-bin

Go로 작성한 single-binary 커뮤니티 엔진입니다.

이 레포는 기능 완성형 제품보다, 아래 설계 개념을 실제 코드로 학습하기 위한 목적으로 구성되어 있습니다.

- Layered Architecture
- Port & Adapter (Hexagonal) 구조
- Domain 중심 설계
- interface - implementation 분리
- 확장 가능한 구조 설계

## 학습 목표

### 1) Layered Architecture
관심사를 레이어 단위로 분리하고, 상위 레이어가 하위 레이어의 구체 구현에 직접 의존하지 않도록 구성합니다.

### 2) Port & Adapter
- Port: `internal/application/useCase.go`, `internal/application/repository.go`
- Adapter: `internal/delivery/http.go`, `internal/infrastructure/persistence/inmemory/*`

핵심 애플리케이션은 Port(interface)만 알고, 실제 입출력(HTTP, In-Memory 저장소)은 Adapter에서 교체 가능하도록 설계했습니다.

### 3) Domain 중심 설계
핵심 데이터 모델(entity)과 반환 모델(dto)을 분리했습니다.

- Entity: `internal/domain/entity/*`
- DTO: `internal/domain/dto/*`

### 4) interface - implementation 분리
`application` 레이어에 인터페이스(계약)를 두고, `service`/`inmemory`에서 구현합니다.

- UseCase 인터페이스: `internal/application/useCase.go`
- Repository 인터페이스: `internal/application/repository.go`
- Service 구현체: `internal/application/service/*`
- InMemory 구현체: `internal/infrastructure/persistence/inmemory/*`

또한 compile-time assertion을 추가해 인터페이스-구현 드리프트를 컴파일 시점에 즉시 검출합니다.

### 5) 확장 가능한 구조
현재는 In-Memory 저장소를 사용하지만, 동일 인터페이스를 구현하는 DB 어댑터를 추가하면 교체할 수 있습니다.
(예: PostgreSQL/MySQL 어댑터)

---

## 현재 아키텍처

요청 흐름:

`HTTP Delivery -> UseCase Port -> Service(Application) -> Repository Port -> InMemory Adapter`

초기 조립(Composition Root):

- `cmd/main.go`
- 서버 실행 포트: `:18577`
- 부팅 시 admin 계정 시드: `admin/admin`

---

## 디렉토리 구조

```txt
cmd/
  main.go                          # 의존성 조립 및 서버 시작

internal/
  delivery/
    http.go                        # HTTP Adapter

  application/
    useCase.go                     # UseCase Port
    repository.go                  # Repository Port
    service/
      userService.go
      boardService.go
      postService.go
      commentService.go
      reactionService.go           # UseCase 구현

  domain/
    entity/
      user.go
      board.go
      post.go
      comment.go
      reaction.go
    dto/
      boardList.go
      postList.go
      postDetail.go
      commentList.go
      commentDetail.go

  infrastructure/
    persistence/
      inmemory/
        userRepository.go
        boardRepository.go
        postRepository.go
        commentRepository.go
        reactionRepository.go      # Repository Adapter 구현

  customError/
    customError.go                 # 공통 에러 정의
```

---

## 도메인 모델

- User: id, name, password, role(admin/user), createdAt
- Board: id, name, description, createdAt
- Post: id, title, content, authorId, boardId, createdAt, updatedAt
- Comment: id, content, authorId, postId, parentId(optional), createdAt
- Reaction: id, targetType(post/comment), targetId, userId, type(like/dislike), createdAt

---

## UseCase 정책(현재 반영 기준)

- User
  - `signUp`, `login`, `logout`, `quit`
- Board
  - `create/update/delete`: admin만 가능
  - `get`: 전체 조회 가능
- Post
  - `create`: 등록 사용자
  - `update/delete`: 작성자 또는 admin
  - `get`: 조회 가능
- Comment
  - `create`: 등록 사용자
  - `update/delete`: 작성자 또는 admin
  - `get`: 조회 가능
- Reaction
  - `add`: 등록 사용자
  - `remove`: 리액션 작성자 또는 admin
  - `get`: 조회 가능

---

## HTTP API

### User
- `POST /users/signup`
- `POST /users/login`
- `POST /users/logout`
- `DELETE /users/quit`

### Board
- `GET /boards?limit=10&offset=0`
- `POST /boards`
- `PUT /boards/{boardID}`
- `DELETE /boards/{boardID}?user_id={userID}`

### Post
- `GET /boards/{boardID}/posts?limit=10&offset=0`
- `POST /boards/{boardID}/posts`
- `GET /posts/{postID}`
- `PUT /posts/{postID}`
- `DELETE /posts/{postID}?author_id={userID}`

### Comment
- `GET /posts/{postID}/comments?limit=10&offset=0`
- `POST /posts/{postID}/comments`
- `PUT /comments/{commentID}`
- `DELETE /comments/{commentID}?author_id={userID}`

### Reaction
- `GET /reactions?target_id={id}&target_type={post|comment}`
- `POST /reactions`
- `DELETE /reactions/{reactionID}?user_id={userID}`

---

## 실행 방법

### 0) Makefile 사용 (권장)

```bash
make help
```

주요 타깃:

- `make build`: 단일 바이너리 빌드 (`bin/commu-bin`)
- `make run`: 빌드 후 실행
- `make test`: 전체 테스트
- `make test-unit`: 단위 테스트만 실행
- `make test-integration`: 통합 테스트만 실행
- `make clean`: 빌드 산출물 삭제

### 1) 실행

```bash
go run ./cmd
```

### 2) 테스트

```bash
go test ./...
```

샌드박스/권한 환경에 따라 Go 캐시 경로가 제한되면 아래처럼 실행할 수 있습니다.

```bash
GOCACHE=/tmp/go-comu-bin-gocache go test ./...
```

### 3) 패키지별 테스트 실행

```bash
go test ./internal/application/service -v
go test ./internal/delivery -v
go test ./internal/infrastructure/persistence/inmemory -v
go test ./internal/domain/entity -v
go test ./internal/integration -v
```

---

## 예시 요청

회원가입:

```bash
curl -X POST http://localhost:18577/users/signup \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}'
```

로그인:

```bash
curl -X POST http://localhost:18577/users/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}'
```

게시판 생성(admin):

```bash
curl -X POST http://localhost:18577/boards \
  -H 'Content-Type: application/json' \
  -d '{"user_id":1,"name":"free","description":"free board"}'
```

---

## 현재 한계와 다음 단계

현재 구조는 인증/인가 미들웨어 도입 전 단계입니다.
그래서 일부 API는 `user_id`/`author_id`를 요청 바디/쿼리로 전달받아 권한을 판단합니다.

다음 단계 권장 순서:

1. 인증 미들웨어 도입 (세션/JWT)
2. 인가 정책 모듈화 (owner/admin 정책 공통화)
3. 저장소 어댑터 교체 가능성 검증 (RDB adapter 추가)
4. 플러그인/이벤트 버스 확장 포인트 정의

---

## 테스트 코드 스타일 가이드

이 프로젝트의 테스트 코드는 아래 글의 방향을 따릅니다.

- 참고: [우리가 테스트를 하는 이유. 근데 이제 Golang을 곁들인](https://blog.banksalad.com/tech/why-we-do-test-by-golang/)

앞으로 테스트를 **추가/수정**할 때도 동일 규칙을 적용해주세요.

### 핵심 원칙

- 가독성을 우선합니다. 테스트를 "동작 문서"처럼 읽을 수 있어야 합니다.
- 실패 지점이 명확해야 합니다. 준비 단계 실패와 검증 실패를 분리합니다.
- 반복되는 패턴은 헬퍼 함수로 추출하되, 테스트 본문의 의도를 숨기지 않습니다.

### Assertion 규칙 (`testify`)

- 기본 assertion 라이브러리는 `github.com/stretchr/testify`를 사용합니다.
- 오류/전제조건 검증은 `require`를 사용합니다.
  - 예: `require.NoError(t, err)`, `require.NotNil(t, got)`
- 결과 값/상태 검증은 `assert`를 사용합니다.
  - 예: `assert.Equal(t, want, got)`, `assert.Len(t, list, 2)`
- 가능하면 `if ... { t.Fatalf(...) }` 패턴 대신 `require/assert`를 우선 사용합니다.

### 테스트 이름/구조

- 테스트 이름은 `Test<대상>_<시나리오>` 형태를 권장합니다.
- 시나리오가 많은 경우 `t.Run("given_..._when_..._then_...")` 형태의 서브테스트를 사용합니다.
- 한 테스트는 하나의 행동/정책을 검증하도록 유지합니다.

### 작성 패턴

- 테스트는 가능한 한 `given -> when -> then` 흐름으로 작성합니다.
- `given`: 데이터/의존성 준비
- `when`: 대상 함수 실행
- `then`: 결과 검증

예시:

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

### PR 체크리스트 (테스트 변경 시)

- 새로 추가한 테스트가 `require/assert` 스타일을 따르는가?
- 실패 메시지 없이도 테스트 의도를 이름과 구조만으로 이해할 수 있는가?
- 정상/오류 경로가 모두 검증되는가?
- `go test ./...`가 통과하는가?

---

## 테스트 구성

현재 테스트는 아래 레벨로 구성되어 있습니다.

- Unit Test
  - `internal/domain/entity`: 엔티티 생성/수정 메서드 검증
  - `internal/infrastructure/persistence/inmemory`: 저장소 CRUD, 필터링, pagination, 경계값 검증
  - `internal/application/service`: 유스케이스 권한/정책/오류 분기 검증
  - `internal/delivery`: HTTP 라우팅, 상태코드, 입력 검증, 에러 매핑 검증
- Integration Test
  - `internal/integration`: HTTP + Service + InMemory를 실제 조합으로 end-to-end 흐름 검증

### 통합 테스트 주요 시나리오

- main flow: admin 로그인 -> board 생성 -> user 회원가입/로그인
- main flow: board 조회 -> post 생성/조회/수정
- main flow: comment 생성/조회/수정
- main flow: comment reaction 생성/조회/삭제 -> comment 삭제
- main flow: post reaction 생성/조회/삭제 -> post 삭제
- main flow: user 로그아웃/탈퇴 -> admin 로그아웃
- forbidden flow: 비관리자의 board 생성 거부
- forbidden flow: 비작성자/비관리자의 post, comment 수정/삭제 거부
- forbidden flow: 비작성자/비관리자의 reaction 삭제 거부
