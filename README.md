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
