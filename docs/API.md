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
- `GET /api/v1/users/{userUUID}/suspension` (인증 필요, admin)
  - 사용자의 현재 제재 상태를 조회합니다.
  - 응답 필드: `user_uuid`, `status`, `reason`, `suspended_until`
- `PUT /api/v1/users/{userUUID}/suspension` (인증 필요, admin)
  - 사용자 쓰기 제재를 설정합니다.
  - 요청 본문: `reason`, `duration`
  - `duration` 허용값: `7d`, `15d`, `30d`, `unlimited`
- `DELETE /api/v1/users/{userUUID}/suspension` (인증 필요, admin)
  - 사용자 쓰기 제재를 해제합니다.

## Board

- `GET /api/v1/boards?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
- `POST /api/v1/boards` (인증 필요, admin)
- `PUT /api/v1/boards/{boardID}` (인증 필요, admin)
- `DELETE /api/v1/boards/{boardID}` (인증 필요, admin)
  - 비어 있는 게시판에만 허용됩니다.
  - 삭제되지 않은 게시글이 하나라도 있으면 `409 Conflict`

## Post

- 상태 모델
  - 내부 기본 상태는 `draft`, `published`, `deleted`
  - 현재 공개 글 생성 API는 임시저장 기능이 없으므로 생성 시 기본 상태는 `published`
  - 삭제 API는 hard delete가 아니라 `deleted` 상태로 전환하는 soft delete 방식
  - 공개 목록/상세 조회에서는 `published`만 노출
- `GET /api/v1/boards/{boardID}/posts?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
  - 게시판이 없으면 `404 Not Found`
- `POST /api/v1/boards/{boardID}/posts` (인증 필요)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `POST /api/v1/boards/{boardID}/posts/drafts` (인증 필요)
  - 임시저장 글을 생성합니다.
  - 생성된 글은 공개 목록/상세에 노출되지 않습니다.
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `GET /api/v1/posts/{postID}`
  - 응답 본문에는 `attachments` 목록이 포함됩니다.
  - `post.content` 안의 이미지 참조는 `![alt](attachment://{attachmentID})` 형식을 사용합니다.
  - 각 attachment는 실제 파일 조회용 `file_url`과 draft 미리보기용 `preview_url`을 포함합니다.
- `POST /api/v1/posts/{postID}/publish` (인증 필요, 작성자 또는 admin)
  - `draft -> published` 상태 전이를 수행합니다.
  - 본문에 포함된 `attachment://{id}` 참조는 실제로 해당 post에 속한 attachment여야 합니다.
- `PUT /api/v1/posts/{postID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - 본문에 포함된 `attachment://{id}` 참조는 실제로 해당 post에 속한 attachment여야 합니다.
- `DELETE /api/v1/posts/{postID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - 하위 댓글은 soft delete 처리됩니다.
  - 첨부는 orphan 처리되어 cleanup job 대상이 됩니다.

## Attachment

- Attachment는 현재 `Post` 전용 메타데이터 도메인입니다.
- 실제 파일 저장은 `FileStorage` 포트를 통해 수행하고, post 연결 메타데이터는 attachment 도메인이 관리합니다.
- 외부 응답 필드: `file_name`, `content_type`, `size_bytes`, `file_url`, `preview_url`
- `storage_key`는 내부 저장 메타데이터로만 유지하고 외부 응답에는 노출하지 않습니다.
- 본문 내 직접 참조 형식: `![alt](attachment://{attachmentID})`
- `GET /api/v1/posts/{postID}/attachments`
  - published post 기준으로 attachment 목록을 조회합니다.
  - orphan attachment는 제외합니다.
- `GET /api/v1/posts/{postID}/attachments/{attachmentID}/file`
  - published post의 attachment 파일 본문을 반환합니다.
  - `attachments[].file_url`이 이 경로를 가리킵니다.
  - `Cache-Control: public, max-age=300`과 `ETag`를 반환합니다.
  - `If-None-Match`가 일치하면 `304 Not Modified`를 반환합니다.
  - orphan attachment는 `404`로 숨깁니다.
- `GET /api/v1/posts/{postID}/attachments/{attachmentID}/preview` (인증 필요, 작성자 또는 admin)
  - draft/published post의 attachment 미리보기 파일 본문을 반환합니다.
  - `attachments[].preview_url` 및 upload 응답의 `preview_url`이 이 경로를 가리킵니다.
  - `Cache-Control: private, no-store`를 반환합니다.
  - orphan attachment도 owner/admin preview에서는 접근할 수 있습니다.
- `POST /api/v1/posts/{postID}/attachments/upload` (인증 필요, 작성자 또는 admin)
  - multipart form의 `file`을 업로드하고 attachment 메타데이터를 함께 생성합니다.
  - 현재는 기존 `draft/published post`에 바로 연결하는 방식입니다.
  - 응답에는 본문에 바로 넣을 수 있는 `embed_markdown`이 포함됩니다.
  - 메타데이터 저장에 실패하면 이미 저장한 파일은 즉시 롤백을 시도합니다.
  - 허용 타입: `image/png`, `image/jpeg`, `image/jpg`, `image/gif`, `image/webp`
  - 최대 크기: `storage.attachment.maxUploadSizeBytes` 설정값
  - 요청의 `Content-Type`은 실제 파일 sniffing 결과와 일치해야 합니다.
  - 서버 내부 저장본은 `storage.attachment.imageOptimization` 설정에 따라 `jpeg/jpg`, `png`를 최적화할 수 있습니다.
  - 저장 키는 같은 파일명 충돌을 피하기 위해 내부적으로 랜덤 suffix를 붙여 생성합니다.
- `DELETE /api/v1/posts/{postID}/attachments/{attachmentID}` (인증 필요, 작성자 또는 admin)
  - attachment 메타데이터와 저장된 파일을 함께 삭제합니다.

## Comment

- 상태 모델
  - 내부 기본 상태는 `active`, `deleted`
  - 삭제 API는 hard delete가 아니라 `deleted` 상태로 전환하는 soft delete 방식
  - 공개 목록/상세 조회에서는 기본적으로 `active` 댓글만 노출한다.
  - 단, 활성 reply가 남아 있는 삭제된 부모 댓글은 `삭제된 댓글` tombstone으로 함께 노출한다.
- 대댓글 규칙
  - 생성 요청에서 `parent_id`를 받는다.
  - 현재 정책은 1-depth 대댓글만 허용한다.
  - 부모 댓글은 같은 게시글에 속한 활성 댓글이어야 한다.
  - 응답은 flat list를 유지하고 `parent_id`로 관계를 표현한다.
- `GET /api/v1/posts/{postID}/comments?limit=10&last_id=0`
  - 응답 메타: `has_more`, `next_last_id`
  - 삭제된 게시글이면 `404 Not Found`
- `POST /api/v1/posts/{postID}/comments` (인증 필요)
  - 요청 본문은 `content`, 선택적 `parent_id`
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `PUT /api/v1/comments/{commentID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
- `DELETE /api/v1/comments/{commentID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - 부모 댓글을 삭제하면 해당 댓글은 `삭제된 댓글` tombstone으로 남습니다.
  - 하위 reply는 그대로 유지되어 계속 조회할 수 있습니다.

## Reaction

- `GET /api/v1/posts/{postID}/reactions`
  - 삭제된 게시글이면 `404 Not Found`
- `PUT /api/v1/posts/{postID}/reactions/me` (인증 필요)
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
  - 대상 게시글이 없으면 `404`
- `DELETE /api/v1/posts/{postID}/reactions/me` (인증 필요)
  - 내 리액션 삭제
  - 리액션이 없어도 `204`
  - 대상 게시글이 없으면 `404`
- `GET /api/v1/comments/{commentID}/reactions`
  - 삭제된 댓글이면 `404 Not Found`
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
curl -X PUT http://localhost:18577/api/v1/users/550e8400-e29b-41d4-a716-446655440000/suspension \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"reason":"spam","duration":"7d"}'
```

### 사용자 제재 조회

```bash
TOKEN="관리자 로그인 응답 Authorization 헤더 값"
curl -X GET http://localhost:18577/api/v1/users/550e8400-e29b-41d4-a716-446655440000/suspension \
  -H "Authorization: $TOKEN"
```

### 게시글 임시저장

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/boards/1/posts/drafts \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"title":"draft title","content":"draft body"}'
```

### 임시저장 글 발행

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/posts/1/publish \
  -H "Authorization: $TOKEN"
```

### 게시글 첨부 메타데이터 조회

```bash
curl -X GET http://localhost:18577/api/v1/posts/1/attachments
```

응답 필드에는 `file_url`이 포함됩니다.

### 게시글 첨부파일 조회

```bash
curl -X GET http://localhost:18577/api/v1/posts/1/attachments/3/file --output a.png
```

### 게시글 첨부파일 업로드

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/posts/1/attachments/upload \
  -H "Authorization: $TOKEN" \
  -F "file=@./a.png"
```

응답 예시:

```json
{
  "id": 3,
  "embed_markdown": "![a.png](attachment://3)",
  "preview_url": "/api/v1/posts/1/attachments/3/preview"
}
```

### 대댓글 작성

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/posts/1/comments \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"content":"reply","parent_id":5}'
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
