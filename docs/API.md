# HTTP API

인증이 필요한 API는 `Authorization: Bearer <token>` 헤더를 사용합니다.
로그인 성공 시 응답 헤더 `Authorization`에 토큰이 반환됩니다.

## OpenAPI / Swagger

- UI: `GET /swagger/index.html`
- 스펙 생성: `make swagger` (`docs/swagger` 산출물 갱신)

모든 엔드포인트는 `/api/v1` prefix를 사용합니다.

## User

- `POST /api/v1/signup`
  - `username`은 유니크해야 하며, 중복 시 `409 Conflict`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout` (인증 필요)
- `DELETE /api/v1/users/me` (인증 필요)

## Board

- `GET /api/v1/boards?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
- `POST /api/v1/boards` (인증 필요, admin)
- `PUT /api/v1/boards/{boardID}` (인증 필요, admin)
- `DELETE /api/v1/boards/{boardID}` (인증 필요, admin)

## Post

- `GET /api/v1/boards/{boardID}/posts?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
- `POST /api/v1/boards/{boardID}/posts` (인증 필요)
- `GET /api/v1/posts/{postID}`
- `PUT /api/v1/posts/{postID}` (인증 필요, 작성자 또는 admin)
- `DELETE /api/v1/posts/{postID}` (인증 필요, 작성자 또는 admin)

## Comment

- `GET /api/v1/posts/{postID}/comments?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
- `POST /api/v1/posts/{postID}/comments` (인증 필요)
- `PUT /api/v1/comments/{commentID}` (인증 필요, 작성자 또는 admin)
- `DELETE /api/v1/comments/{commentID}` (인증 필요, 작성자 또는 admin)

## Reaction

- `GET /api/v1/posts/{postID}/reactions`
- `PUT /api/v1/posts/{postID}/reactions/me` (인증 필요)
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
- `DELETE /api/v1/posts/{postID}/reactions/me` (인증 필요)
  - 내 리액션 삭제
  - 리액션이 없어도 `204`
- `GET /api/v1/comments/{commentID}/reactions`
- `PUT /api/v1/comments/{commentID}/reactions/me` (인증 필요)
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
- `DELETE /api/v1/comments/{commentID}/reactions/me` (인증 필요)
  - 내 리액션 삭제
  - 리액션이 없어도 `204`

`reaction_type` 요청 값은 현재 `like`, `dislike` 를 지원합니다.

## 예시 요청

### 회원가입

```bash
curl -X POST http://localhost:18577/api/v1/signup \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}'
```

### 로그인

```bash
curl -X POST http://localhost:18577/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}'
```

### 게시판 생성

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/boards \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"name":"free","description":"free board"}'
```

### 게시글 내 리액션 생성

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X PUT http://localhost:18577/api/v1/posts/1/reactions/me \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"reaction_type":"like"}'
```

### 게시글 내 리액션 변경

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X PUT http://localhost:18577/api/v1/posts/1/reactions/me \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"reaction_type":"dislike"}'
```

### 게시글 내 리액션 삭제

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X DELETE http://localhost:18577/api/v1/posts/1/reactions/me \
  -H "Authorization: $TOKEN"
```
