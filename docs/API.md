# HTTP API

인증이 필요한 API는 `Authorization: Bearer <token>` 헤더를 사용합니다.
로그인 성공 시 응답 헤더 `Authorization`에 토큰이 반환됩니다.

## User

- `POST /users/signup`
- `POST /users/login`
- `POST /users/logout` (인증 필요)
- `DELETE /users/quit`

## Board

- `GET /boards?limit=10&offset=0`
- `POST /boards` (인증 필요, admin)
- `PUT /boards/{boardID}` (인증 필요, admin)
- `DELETE /boards/{boardID}` (인증 필요, admin)

## Post

- `GET /boards/{boardID}/posts?limit=10&offset=0`
- `POST /boards/{boardID}/posts` (인증 필요)
- `GET /posts/{postID}`
- `PUT /posts/{postID}` (인증 필요, 작성자 또는 admin)
- `DELETE /posts/{postID}` (인증 필요, 작성자 또는 admin)

## Comment

- `GET /posts/{postID}/comments?limit=10&offset=0`
- `POST /posts/{postID}/comments` (인증 필요)
- `PUT /comments/{commentID}` (인증 필요, 작성자 또는 admin)
- `DELETE /comments/{commentID}` (인증 필요, 작성자 또는 admin)

## Reaction

- `GET /reactions?target_id={id}&target_type={post|comment}`
- `POST /reactions` (인증 필요)
- `DELETE /reactions/{reactionID}` (인증 필요, 작성자 또는 admin)

## 예시 요청

### 회원가입

```bash
curl -X POST http://localhost:18577/users/signup \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}'
```

### 로그인

```bash
curl -X POST http://localhost:18577/users/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pw"}'
```

### 게시판 생성

```bash
TOKEN="로그인 응답 Authorization 헤더"
curl -X POST http://localhost:18577/boards \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"name":"free","description":"free board"}'
```
