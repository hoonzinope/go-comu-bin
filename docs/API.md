# HTTP API

인증이 필요한 API는 `Authorization: Bearer <token>` 헤더를 사용합니다.
로그인 성공 시 응답 헤더 `Authorization`에 `Bearer <token>` 형식으로 토큰이 반환됩니다.
`Bearer` 스킴이 아니면 인증 실패(`401`)로 처리합니다.
JSON 요청 바디는 `delivery.http.maxJSONBodyBytes`를 초과하면 `400 Bad Request`로 거부합니다.
`delivery.http.rateLimit.enabled=true`이면 `/api/v1` 하위 read/write 요청에 IP 기준 rate limit이 적용되며, 초과 시 `429 Too Many Requests`를 반환합니다.
기본 서버는 trusted proxy를 신뢰하지 않으므로, 별도 reverse proxy trust 설정이 없으면 forwarded header는 client IP 판별에 사용하지 않습니다.
텍스트 입력은 markdown 원문을 보존하기 위해 저장 전에 변형하지 않습니다. XSS 방어는 렌더링 경계의 escaping/sanitizing 책임으로 둡니다.

## OpenAPI / Swagger

- UI: `GET /swagger/index.html`
- 스펙 생성: `make swagger` (`docs/swagger` 산출물 갱신)
- 정합성 검증: `make verify` 또는 `./scripts/verify-swagger.sh`

모든 엔드포인트는 `/api/v1` prefix를 사용합니다.

공개 도메인 리소스의 외부 식별자는 UUID를 사용합니다.
- 대상: `User`, `Board`, `Post`, `Comment`, `Attachment`
- 예: `author_uuid`, `user_uuid`, `board_uuid`, `post_uuid`, `target_uuid`

운영 리소스는 예외적으로 추적용 식별자를 유지합니다.
- `reportID`: admin 신고 처리 대상
- `messageID`: dead outbox 메시지 대상

페이지네이션도 같은 원칙을 따릅니다.
- 공개 목록 API: opaque `cursor`
- 운영 목록 API(`admin/reports`, `admin/outbox/dead`): 추적용 `last_id`

## User

- `POST /api/v1/signup`
  - 요청 본문: `username`, `email`, `password`
  - `username`은 유니크해야 하며, 중복 시 `409 Conflict`
  - `email`도 유니크해야 하며, 형식 오류는 `400 Bad Request`
  - 성공 시 email verification token을 자동 발급해 mail sender 경로로 전달합니다.
- `POST /api/v1/auth/guest`
  - 서버가 내부 규칙으로 guest 계정을 생성하고 즉시 bearer token을 발급합니다.
  - guest는 일반 사용자와 동일하게 `id`, `uuid`, 세션을 가지지만, 외부에는 guest용 내부 식별자를 노출하지 않습니다.
  - guest는 일반 signup 대상이 아니며, 브라우저 최초 방문 시 토큰 발급 용도로만 사용합니다.
  - 내부 lifecycle은 `pending -> active -> expired`를 사용하며, session 저장이 완료된 `active guest`만 인증/쓰기 대상이 됩니다.
- `POST /api/v1/auth/login`
  - 사용자 미존재 또는 비밀번호 불일치 시 동일하게 `401 Unauthorized`
  - guest 계정은 username/password 로그인 대상이 아닙니다.
- `POST /api/v1/auth/guest/upgrade` (인증 필요)
  - 현재 bearer token 소유자가 guest 계정일 때만 정식 계정으로 승격합니다.
  - 요청 본문: `username`, `email`, `password`
  - 승격 시 기존 `id`, `uuid`, 작성물 소유권은 유지됩니다.
  - 성공 시 응답 헤더 `Authorization`에 새 `Bearer <token>`을 반환하고, 기존 guest token은 즉시 폐기합니다.
  - 새 bearer token 발급과 기존 guest token 폐기가 함께 완료된 경우에만 승격 성공으로 간주합니다.
  - 승격 성공 시 email verification token을 자동 발급해 mail sender 경로로 전달합니다.
  - guest가 아닌 사용자가 호출하면 `400 Bad Request`
- `POST /api/v1/auth/email-verification/request` (인증 필요)
  - 현재 로그인 사용자 기준으로 email verification token을 새로 발급합니다.
  - 이미 인증된 사용자도 동일한 성공 응답으로 처리합니다.
- `POST /api/v1/auth/email-verification/confirm`
  - 요청 본문: `token`
  - token이 유효하면 사용자 email을 verified 상태로 전환합니다.
  - 유효하지 않거나 만료되었거나 이미 사용된 token은 동일한 공개 에러로 처리합니다.
- email verification이 필요한 쓰기 기능
  - `attachment` 업로드/삭제
  - `report` 생성
  - 일반 미인증 사용자의 `post/comment/reaction`은 허용됩니다.
  - guest는 기존 정책대로 `post/comment`만 허용되고, `reaction/attachment/report`는 사용할 수 없습니다.
- `POST /api/v1/auth/password-reset/request`
  - 요청 본문: `email`
  - email 형식 오류만 `400 Bad Request`로 처리합니다.
  - 존재하지 않는 email, guest 계정, soft-deleted 계정 여부는 동일한 성공 응답으로 숨깁니다.
  - 전용 rate limit이 `client_ip + normalized_email` 기준으로 적용되며, 초과 시 `429 Too Many Requests`를 반환합니다.
  - reset token은 API 응답에 포함하지 않고 mail sender 경로로만 전달합니다.
  - SMTP가 활성화된 환경에서는 frontend reset 페이지 링크와 fallback token이 함께 메일에 포함됩니다.
- `POST /api/v1/auth/password-reset/confirm`
  - 요청 본문: `token`, `new_password`
  - token이 유효하면 비밀번호를 변경하고 기존 활성 세션을 모두 무효화합니다.
  - 유효하지 않거나 만료되었거나 이미 사용된 token은 동일한 공개 에러로 처리합니다.
- `POST /api/v1/auth/logout` (인증 필요)
- `DELETE /api/v1/users/me` (인증 필요)
  - 계정은 soft delete 처리되고, 식별 정보는 익명화됩니다.
  - 탈퇴 성공 시 해당 사용자의 활성 세션 무효화를 시도합니다.
  - 세션 정리는 best effort로 처리되며, 계정 삭제 성공이 우선됩니다.
  - guest 계정은 self-delete를 허용하지 않으며 `403 Forbidden`을 반환합니다.
  - 내부 정리 대상 guest는 background cleanup job이 soft delete 처리합니다.
- `GET /api/v1/users/me/notifications?limit=10&cursor=` (인증 필요)
  - 자신의 notification inbox 목록을 조회합니다.
  - 응답 메타: `has_more`, `next_cursor`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
  - 응답 필드: `uuid`, `type`, `actor_uuid`, `post_uuid`, `comment_uuid`, `actor_name`, `post_title`, `comment_preview`, `read_at`, `created_at`
  - v1 알림 타입은 `post_commented`, `comment_replied`, `mentioned` 입니다.
  - snapshot은 저장 시점 기준 최소 스냅샷을 사용하며, 현재 길이는 `post_title` 50자, `comment_preview` 50자입니다.
- `GET /api/v1/users/me/notifications/unread-count` (인증 필요)
  - 자신의 unread notification 개수를 조회합니다.
- `PATCH /api/v1/users/me/notifications/{notificationUUID}/read` (인증 필요)
  - 자신의 notification 한 건을 읽음 처리합니다.
  - 이미 읽은 notification을 다시 호출해도 성공(`204`)으로 처리합니다.
  - 다른 사용자의 notification UUID는 `404 Not Found`로 숨깁니다.
- `GET /api/v1/users/{userUUID}/suspension` (인증 필요, admin)
  - 사용자의 현재 제재 상태를 조회합니다.
  - `userUUID`는 유효한 UUID 형식이어야 하며, 형식 오류는 `400 Bad Request`
  - 응답 필드: `user_uuid`, `status`, `reason`, `suspended_until`
- `PUT /api/v1/users/{userUUID}/suspension` (인증 필요, admin)
  - 사용자 쓰기 제재를 설정합니다.
  - `userUUID`는 유효한 UUID 형식이어야 하며, 형식 오류는 `400 Bad Request`
  - 요청 본문: `reason`, `duration`
  - `duration` 허용값: `7d`, `15d`, `30d`, `unlimited`
- `DELETE /api/v1/users/{userUUID}/suspension` (인증 필요, admin)
  - 사용자 쓰기 제재를 해제합니다.
  - `userUUID`는 유효한 UUID 형식이어야 하며, 형식 오류는 `400 Bad Request`

## Report

- `POST /api/v1/reports` (인증 필요)
  - 신고를 생성합니다.
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - email 미인증 사용자는 사용할 수 없습니다(`403 Forbidden`).
  - 생성 응답은 운영 리소스 식별자인 숫자 `id`를 반환합니다.
  - 요청 본문: `target_type`, `target_uuid`, `reason_code`, 선택적 `reason_detail`
  - `target_type` 허용값: `post`, `comment`
  - `reason_code` 허용값: `spam`, `abuse`, `sexual`, `violence`, `illegal`, `other`
  - 동일 사용자는 동일 대상(`target_type`, `target_uuid`)을 한 번만 신고할 수 있습니다. 중복 신고는 `409 Conflict`
  - hidden 게시판의 대상은 non-admin에게 기존 공개 조회와 동일하게 `not found`로 처리합니다.

## Admin Operations

- `GET /api/v1/admin/reports?status=&limit=10&last_id=0` (인증 필요, admin)
  - 신고 목록을 조회합니다.
  - 운영 목록 API이므로 `last_id` 기반 pagination을 유지합니다.
  - 기본 정렬은 `pending` 우선 + 최신순입니다.
  - `status` 필터는 선택값이며, 허용값은 `pending`, `accepted`, `rejected`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
  - 신고 리소스 자체의 식별자는 숫자 `id`를 유지합니다.
  - 응답 참조 필드는 UUID 중심입니다: `target_uuid`, `reporter_uuid`, `resolved_by_uuid`
  - 내부 FK인 `target_id`, `reporter_user_id`, `resolved_by`는 외부 응답에 노출하지 않습니다.
- `PUT /api/v1/admin/reports/{reportID}/resolve` (인증 필요, admin)
  - 신고를 수동 처리합니다.
  - `reportID`는 운영 리소스 식별자이며 숫자 ID를 유지합니다.
  - 요청 본문: `status`, 선택적 `resolution_note`
  - `status` 허용값: `accepted`, `rejected`
  - 신고 처리만 수행하며, 콘텐츠 숨김/유저 제재는 자동 적용하지 않습니다.
- `GET /api/v1/admin/outbox/dead?limit=10&last_id=` (인증 필요, admin)
  - dead outbox 메시지 목록을 조회합니다.
  - 운영 목록 API이므로 `last_id` 기반 pagination을 유지합니다.
  - 응답 최소 필드: `id`, `event_name`, `attempt_count`, `last_error`, `occurred_at`, `next_attempt_at`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
- `POST /api/v1/admin/outbox/dead/{messageID}/requeue` (인증 필요, admin)
  - dead 메시지를 재처리 큐로 되돌립니다.
  - `messageID`는 운영 추적용 식별자입니다.
- `DELETE /api/v1/admin/outbox/dead/{messageID}` (인증 필요, admin)
  - dead 메시지를 폐기(discard)합니다.
  - `messageID`는 운영 추적용 식별자입니다.
- `PUT /api/v1/admin/boards/{boardUUID}/visibility` (인증 필요, admin)
  - 게시판 `hidden` 상태를 변경합니다.
  - 요청 본문: `hidden` (`true|false`)

## Board

- `GET /api/v1/boards?limit=10&cursor=`
  - 응답 메타: `has_more`, `next_cursor`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
- `POST /api/v1/boards` (인증 필요, admin)
- `PUT /api/v1/boards/{boardUUID}` (인증 필요, admin)
- `DELETE /api/v1/boards/{boardUUID}` (인증 필요, admin)
  - 비어 있는 게시판에만 허용됩니다.
  - 삭제되지 않은 게시글이 하나라도 있으면 `409 Conflict`
  - `hidden` 게시판은 비admin에게 완전 비노출됩니다.

## Post

- 상태 모델
  - 내부 기본 상태는 `draft`, `published`, `deleted`
  - 현재 공개 글 생성 API는 임시저장 기능이 없으므로 생성 시 기본 상태는 `published`
  - 삭제 API는 hard delete가 아니라 `deleted` 상태로 전환하는 soft delete 방식
  - 공개 목록/상세 조회에서는 `published`만 노출
- `GET /api/v1/boards/{boardUUID}/posts?limit=10&cursor=`
  - 응답 메타: `has_more`, `next_cursor`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
  - 게시판이 없으면 `404 Not Found`
- `GET /api/v1/posts/search?q=go+search&limit=10&cursor=`
  - 응답 메타: `has_more`, `next_cursor`
  - `q`는 필수이며, 앞뒤 공백 제거 후 비어 있으면 `400 Bad Request`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
  - 검색 대상은 `published` post의 `title`, `content`, `tags` 입니다.
  - 검색 토큰화는 한국어/영어 구분 없이 공백 기준이며, 현재는 모든 토큰이 매치되어야 결과에 포함됩니다.
  - 검색 인덱스는 `post.changed` 이벤트를 소비해 비동기로 갱신되므로, write 직후 짧은 eventual consistency 구간이 있을 수 있습니다.
  - 런타임 인덱스 rebuild가 실행되더라도, rebuild 시작 이후 반영된 더 최신 개별 post 인덱스 갱신은 덮어쓰지 않습니다.
  - hidden 게시판의 post는 비admin 공개 검색 결과에서 제외됩니다.
  - 응답 본문은 기존 post 목록과 동일한 `PostList` shape를 사용하며, relevance score/snippet은 외부에 노출하지 않습니다.
- `POST /api/v1/boards/{boardUUID}/posts` (인증 필요)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정도 작성할 수 있습니다.
  - 생성 시점 본문에는 `attachment://{attachmentUUID}` 참조를 포함할 수 없습니다.
  - 첨부가 필요한 글은 먼저 draft를 만든 뒤 첨부 업로드와 본문 수정을 거쳐 publish 해야 합니다.
  - 요청 본문은 선택적 `tags`, `mentioned_usernames` 배열을 받을 수 있습니다.
  - `tags`는 최대 10개, 각 항목 최대 30자이며, 앞뒤 공백 제거 후 영문 소문자로 정규화됩니다.
  - `mentioned_usernames`는 FE mention UI가 명시적으로 구성한 대상 목록만 받습니다.
  - backend는 `mentioned_usernames`에 포함된 존재하는 사용자만 `mentioned` notification 대상으로 삼습니다.
- `POST /api/v1/boards/{boardUUID}/posts/drafts` (인증 필요)
  - 임시저장 글을 생성합니다.
  - 생성된 글은 공개 목록/상세에 노출되지 않습니다.
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - 생성 시점 본문에는 `attachment://{attachmentUUID}` 참조를 포함할 수 없습니다.
  - 요청 본문은 선택적 `tags` 배열을 받을 수 있습니다.
- `GET /api/v1/posts/{postUUID}`
  - 응답 본문에는 `tags` 목록이 포함됩니다.
  - `tags[]`는 공개 조회용 메타데이터이며 현재 `name`, `created_at` 중심으로 사용합니다.
  - 응답 본문에는 `attachments` 목록이 포함됩니다.
  - 응답의 `comments` 는 최신 공개 댓글 최대 10개만 포함합니다.
  - `comments_has_more=true` 면 상세에 포함되지 않은 추가 댓글이 더 있다는 뜻입니다.
  - 댓글 전체 목록이 필요하면 `GET /api/v1/posts/{postUUID}/comments` 를 사용합니다.
  - `post.content` 안의 이미지 참조는 `![alt](attachment://{attachmentUUID})` 형식을 사용합니다.
  - 각 attachment는 실제 파일 조회용 `file_url`과 draft 미리보기용 `preview_url`을 포함합니다.
  - `reactions[]`는 `target_uuid`, `user_uuid`, `type`을 기준으로 해석합니다.
- `POST /api/v1/posts/{postUUID}/publish` (인증 필요, 작성자 또는 admin)
  - `draft -> published` 상태 전이를 수행합니다.
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - 본문에 포함된 `attachment://{attachmentUUID}` 참조는 실제로 해당 post에 속한 attachment여야 합니다.
- `PUT /api/v1/posts/{postUUID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정도 수정할 수 있습니다.
  - 본문에 포함된 `attachment://{attachmentUUID}` 참조는 실제로 해당 post에 속한 attachment여야 합니다.
  - 요청 본문은 선택적 `tags` 배열을 받을 수 있습니다.
- `DELETE /api/v1/posts/{postUUID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정도 삭제할 수 있습니다.
  - 하위 댓글은 soft delete 처리됩니다.
  - 첨부는 orphan 처리되어 cleanup job 대상이 됩니다.

## Tag

- `GET /api/v1/tags/{tagName}/posts?limit=10&cursor=`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
  - 응답 메타: `has_more`, `next_cursor`
  - `tagName`은 내부적으로 앞뒤 공백 제거 후 영문 소문자로 정규화합니다.
  - tag가 없으면 `404 Not Found`
  - post 목록 응답에는 태그 정보가 포함되지 않습니다.
  - 공개 식별자는 `tagName`을 사용하며, 운영용 숫자 tag ID를 외부 계약의 기본 식별자로 취급하지 않습니다.

## Attachment

- Attachment는 현재 `Post` 전용 메타데이터 도메인입니다.
- 실제 파일 저장은 `FileStorage` 포트를 통해 수행하고, post 연결 메타데이터는 attachment 도메인이 관리합니다.
- 외부 응답 필드: `file_name`, `content_type`, `size_bytes`, `file_url`, `preview_url`
- `storage_key`는 내부 저장 메타데이터로만 유지하고 외부 응답에는 노출하지 않습니다.
- 본문 내 직접 참조 형식: `![alt](attachment://{attachmentUUID})`
- `GET /api/v1/posts/{postUUID}/attachments`
  - published post 기준으로 attachment 목록을 조회합니다.
  - orphan attachment와 `pending_delete` attachment는 제외합니다.
- `GET /api/v1/posts/{postUUID}/attachments/{attachmentUUID}/file`
  - published post의 attachment 파일 본문을 반환합니다.
  - `attachments[].file_url`이 이 경로를 가리킵니다.
  - revoke 이후 stale public cache가 남지 않도록 `Cache-Control: no-store`와 `ETag`를 반환합니다.
  - `If-None-Match`가 일치하면 `304 Not Modified`를 반환합니다.
  - orphan attachment와 `pending_delete` attachment는 `404`로 숨깁니다.
- `GET /api/v1/posts/{postUUID}/attachments/{attachmentUUID}/preview` (인증 필요, 작성자 또는 admin)
  - draft/published post의 attachment 미리보기 파일 본문을 반환합니다.
  - `attachments[].preview_url` 및 upload 응답의 `preview_url`이 이 경로를 가리킵니다.
  - `Cache-Control: private, no-store`를 반환합니다.
  - orphan attachment는 owner/admin preview에서는 접근할 수 있습니다.
  - `pending_delete` attachment는 owner/admin preview에서도 접근할 수 없습니다.
- `POST /api/v1/posts/{postUUID}/attachments/upload` (인증 필요, 작성자 또는 admin)
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - multipart form의 `file`을 업로드하고 attachment 메타데이터를 함께 생성합니다.
  - 현재는 기존 `draft/published post`에 바로 연결하는 방식입니다.
  - 응답에는 본문에 바로 넣을 수 있는 `embed_markdown`이 포함됩니다.
  - `embed_markdown`의 alt text는 filename을 그대로 노출하되, Markdown-safe 형태로 escape 됩니다.
  - 메타데이터 저장에 실패하면 이미 저장한 파일은 즉시 롤백을 시도합니다.
  - 허용 타입: `image/png`, `image/jpeg`, `image/jpg`, `image/gif`, `image/webp`
  - 최대 크기: `storage.attachment.maxUploadSizeBytes` 설정값
  - 요청의 `Content-Type`은 실제 파일 sniffing 결과와 일치해야 합니다.
  - 서버 내부 저장본은 `storage.attachment.imageOptimization` 설정에 따라 `jpeg/jpg`, `png`를 최적화할 수 있습니다.
  - 저장 키는 같은 파일명 충돌을 피하기 위해 내부적으로 랜덤 suffix를 붙여 생성합니다.
- `DELETE /api/v1/posts/{postUUID}/attachments/{attachmentUUID}` (인증 필요, 작성자 또는 admin)
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - attachment는 즉시 `pending_delete` 상태로 숨겨지고, 실제 파일/메타데이터 hard delete는 cleanup job이 비동기로 수행합니다.
  - 게시글 본문에서 `attachment://{attachmentUUID}` 로 참조 중인 첨부는 삭제할 수 없습니다.

## Comment

- `GET /api/v1/posts/{postUUID}/comments?limit=10&cursor=`
  - 응답 메타: `has_more`, `next_cursor`
  - 삭제된 부모 댓글에 활성 reply가 남아 있으면 tombstone으로 함께 노출할 수 있습니다.
- `POST /api/v1/posts/{postUUID}/comments` (인증 필요)
  - guest 계정도 작성할 수 있습니다.
  - 요청 본문: `content`, 선택적 `parent_uuid`, 선택적 `mentioned_usernames`
  - `parent_uuid`를 주면 1-depth reply를 생성합니다. reply의 parent는 같은 post의 활성 top-level comment여야 합니다.
  - top-level comment는 post 작성자에게 `post_commented` notification을 비동기로 생성합니다.
  - reply는 부모 comment 작성자에게 `comment_replied` notification을 비동기로 생성합니다.
  - mention 알림은 본문 raw text를 직접 파싱하지 않고, FE가 명시적으로 넘긴 `mentioned_usernames` 목록만 사용합니다.
  - 자기 자신에게 향하는 notification은 생성하지 않습니다.

- 상태 모델
  - 내부 기본 상태는 `active`, `deleted`
  - 삭제 API는 hard delete가 아니라 `deleted` 상태로 전환하는 soft delete 방식
  - 공개 목록/상세 조회에서는 기본적으로 `active` 댓글만 노출한다.
  - 단, 활성 reply가 남아 있는 삭제된 부모 댓글은 `삭제된 댓글` tombstone으로 함께 노출한다.
- 대댓글 규칙
  - 생성 요청에서 `parent_uuid`를 받는다.
  - 현재 정책은 1-depth 대댓글만 허용한다.
  - 부모 댓글은 같은 게시글에 속한 활성 댓글이어야 한다.
  - 응답은 flat list를 유지하고 `parent_uuid`로 관계를 표현한다.
- `GET /api/v1/posts/{postUUID}/comments?limit=10&cursor=`
  - 응답 메타: `has_more`, `next_cursor`
  - `limit`은 `1..1000` 범위의 정수여야 합니다.
  - 삭제된 게시글이면 `404 Not Found`
- `POST /api/v1/posts/{postUUID}/comments` (인증 필요)
  - 요청 본문은 `content`, 선택적 `parent_uuid`
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정도 작성할 수 있습니다.
- `PUT /api/v1/comments/{commentUUID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정도 수정할 수 있습니다.
- `DELETE /api/v1/comments/{commentUUID}` (인증 필요, 작성자 또는 admin)
  - 정지된(`suspended`) 사용자는 `403 Forbidden`
  - guest 계정도 삭제할 수 있습니다.
  - 부모 댓글을 삭제하면 해당 댓글은 `삭제된 댓글` tombstone으로 남습니다.
  - 하위 reply는 그대로 유지되어 계속 조회할 수 있습니다.

## Reaction

- `GET /api/v1/posts/{postUUID}/reactions`
  - 삭제된 게시글이면 `404 Not Found`
- `PUT /api/v1/posts/{postUUID}/reactions/me` (인증 필요)
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
  - 대상 게시글이 없으면 `404`
- `DELETE /api/v1/posts/{postUUID}/reactions/me` (인증 필요)
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - 내 리액션 삭제
  - 리액션이 없어도 `204`
  - 대상 게시글이 없으면 `404`
- `GET /api/v1/comments/{commentUUID}/reactions`
  - 삭제된 댓글이면 `404 Not Found`
- `PUT /api/v1/comments/{commentUUID}/reactions/me` (인증 필요)
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - 내 리액션 생성 또는 변경
  - 없으면 생성(`201`), 있으면 변경 또는 no-op(`204`)
  - 대상 댓글이 없으면 `404`
- `DELETE /api/v1/comments/{commentUUID}/reactions/me` (인증 필요)
  - guest 계정은 사용할 수 없습니다(`403 Forbidden`).
  - 내 리액션 삭제
  - 리액션이 없어도 `204`
  - 대상 댓글이 없으면 `404`

`reaction_type` 요청 값은 현재 `like`, `dislike` 를 지원합니다.

## 예시 요청

### 회원가입

```bash
curl -X POST http://localhost:18577/api/v1/signup \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.com","password":"pw"}'
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
curl -X POST http://localhost:18577/api/v1/boards/550e8400-e29b-41d4-a716-446655440001/posts/drafts \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"title":"draft title","content":"draft body"}'
```

### 임시저장 글 발행

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/publish \
  -H "Authorization: $TOKEN"
```

### 게시글 첨부 메타데이터 조회

```bash
curl -X GET http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/attachments
```

응답 필드에는 `file_url`이 포함됩니다.

### 게시글 첨부파일 조회

```bash
curl -X GET http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/attachments/550e8400-e29b-41d4-a716-446655440003/file --output a.png
```

### 게시글 첨부파일 업로드

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/attachments/upload \
  -H "Authorization: $TOKEN" \
  -F "file=@./a.png"
```

응답 예시:

```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440003",
  "embed_markdown": "![a.png](attachment://550e8400-e29b-41d4-a716-446655440003)",
  "preview_url": "/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/attachments/550e8400-e29b-41d4-a716-446655440003/preview"
}
```

### 대댓글 작성

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X POST http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/comments \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"content":"reply","parent_uuid":"550e8400-e29b-41d4-a716-446655440004"}'
```

### 게시글 내 리액션 생성

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X PUT http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/reactions/me \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"reaction_type":"like"}'
```

### 게시글 내 리액션 변경

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X PUT http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/reactions/me \
  -H "Content-Type: application/json" \
  -H "Authorization: $TOKEN" \
  -d '{"reaction_type":"dislike"}'
```

### 게시글 내 리액션 삭제

```bash
TOKEN="로그인 응답 Authorization 헤더 값"
curl -X DELETE http://localhost:18577/api/v1/posts/550e8400-e29b-41d4-a716-446655440002/reactions/me \
  -H "Authorization: $TOKEN"
```
