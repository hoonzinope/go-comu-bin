# HTTP API

인증이 필요한 API는 `Authorization: Bearer <token>` 헤더를 사용합니다.
로그인 성공 시 응답 헤더 `Authorization`에 토큰이 반환됩니다.

모든 엔드포인트는 `/api/v1` prefix를 사용합니다.

## User

- `POST /api/v1/signup`
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
- `POST /api/v1/posts/{postID}/reactions` (인증 필요)
- `GET /api/v1/comments/{commentID}/reactions`
- `POST /api/v1/comments/{commentID}/reactions` (인증 필요)
- `DELETE /api/v1/reactions/{reactionID}` (인증 필요, 작성자 또는 admin)

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
TOKEN="로그인 응답 Authorization 헤더"
curl -X POST http://localhost:18577/api/v1/boards \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"name":"free","description":"free board"}'
```
