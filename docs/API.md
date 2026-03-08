# HTTP API

인증이 필요한 API는 `Authorization: Bearer <token>` 헤더를 사용합니다.
로그인 성공 시 응답 헤더 `Authorization`에 `Bearer <token>` 형식으로 토큰이 반환됩니다.
`Bearer` 스킴이 아니면 인증 실패(`401`)로 처리합니다.

## OpenAPI / Swagger

- UI: `GET /swagger/index.html`
- 스펙 생성: `make swagger` (`docs/swagger` 산출물 갱신)

모든 엔드포인트는 `/api/v1` prefix를 사용합니다.

공개 응답의 사용자 식별자는 내부 `id` 대신 `uuid`를 사용합니다.
예: `author_uuid`, `user_uuid`

## User

- `POST /api/v1/signup`
  - `username`은 유니크해야 하며, 중복 시 `409 Conflict`
- `POST /api/v1/auth/login`
  - 사용자 미존재 또는 비밀번호 불일치 시 동일하게 `401 Unauthorized`
- `POST /api/v1/auth/logout` (인증 필요)
- `DELETE /api/v1/users/me` (인증 필요)
  - 계정은 soft delete 처리되고, 식별 정보는 익명화됩니다.
  - 탈퇴 성공 시 해당 사용자의 활성 세션 무효화를 시도합니다.
  - 세션 정리는 best effort로 처리되며, 계정 삭제 성공이 우선됩니다.
- `GET /api/v1/users/{userID}/suspension` (인증 필요, admin)
  - 사용자의 현재 제재 상태를 조회합니다.
  - 응답 필드: `user_id`, `status`, `reason`, `suspended_until`
- `PUT /api/v1/users/{userID}/suspension` (인증 필요, admin)
  - 사용자 쓰기 제재를 설정합니다.
  - 요청 본문: `reason`, `duration`
  - `duration` 허용값: `7d`, `15d`, `30d`, `unlimited`
- `DELETE /api/v1/users/{userID}/suspension` (인증 필요, admin)
  - 사용자 쓰기 제재를 해제합니다.

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
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `GET /api/v1/posts/{postID}`
- `PUT /api/v1/posts/{postID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `DELETE /api/v1/posts/{postID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`

## Comment

- `GET /api/v1/posts/{postID}/comments?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
- `POST /api/v1/posts/{postID}/comments` (인증 필요)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `PUT /api/v1/comments/{commentID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `DELETE /api/v1/comments/{commentID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`

## Reaction

- `GET /api/v1/posts/{postID}/reactions`
- `PUT /api/v1/posts/{postID}/reactions/me` (인증 필요)
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
  - 대상 게시글이 없으면 `404`
- `DELETE /api/v1/posts/{postID}/reactions/me` (인증 필요)
  - 내 리액션 삭제
  - 리액션이 없어도 `204`
  - 대상 게시글이 없으면 `404`
- `GET /api/v1/comments/{commentID}/reactions`
- `PUT /api/v1/comments/{commentID}/reactions/me` (인증 필요)
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
  - 대상 댓글이 없으면 `404`
- `DELETE /api/v1/comments/{commentID}/reactions/me` (인증 필요)
  - 내 리액션 삭제
  - 리액션이 없어도 `204`
  - 대상 댓글이 없으면 `404`

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

### 사용자 제재 설정

```bash
TOKEN="관리자 로그인 응답 Authorization 헤더 값"
curl -X PUT http://localhost:18577/api/v1/users/2/suspension \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"reason":"spam","duration":"7d"}'
```

### 사용자 제재 조회

```bash
TOKEN="관리자 로그인 응답 Authorization 헤더 값"
curl -X GET http://localhost:18577/api/v1/users/2/suspension \
  -H "Authorization: $TOKEN"
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
