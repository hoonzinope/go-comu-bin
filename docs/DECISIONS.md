# Decisions Log

이 문서는 프로젝트 운영 중 나온 특정 사안에 대한 논의 배경, 판단 기준, 결론, 후속 액션을 시간순으로 기록하기 위한 히스토리 문서입니다.

목적

- 일회성 대화로 끝나지 않도록 중요한 판단을 남긴다.
- 왜 그런 결정을 했는지 배경과 제약을 같이 기록한다.
- 이후 설계 변경 시 과거 판단을 비교할 수 있게 한다.

기록 원칙

- 새로운 논의가 생기면 아래 템플릿으로 항목을 추가한다.
- 결론이 보류된 경우에도 현재 가설과 보류 사유를 남긴다.
- 구현 완료 여부와 별개로, "왜 이 방향을 선택했는가"를 우선 기록한다.
- 되도록 관련 문서/코드 경로를 같이 적는다.

템플릿

```md
## YYYY-MM-DD - 주제

상태

- proposed | decided | superseded

배경

- 이 논의가 왜 필요했는지

관찰

- 현재 코드/문서 기준 사실

결론

- 최종 판단

후속 작업

- 필요한 작업 목록

관련 문서/코드

- docs/...
- internal/...
```

## 2026-03-08 - 도메인 도입 우선순위 및 기존 엔티티 보강 방향

상태

- decided

배경

- `docs/ROADMAP.md`에는 Step 2 확장 도메인으로 `Attachment`, `Report`, `Notification`, `PointHistory`, `Tag`가 정의되어 있다.
- 현재 레포는 `user`, `board`, `post`, `comment`, `reaction`, `session`, `account` 중심으로 구성되어 있다.
- 신규 도메인을 추가하기 전에, 현재 코어 엔티티가 운영성 요구를 얼마나 수용할 수 있는지 점검할 필요가 있었다.

관찰

- `User`는 soft delete + 익명화는 지원하지만, 이메일 인증/비밀번호 재설정/정지 같은 생명주기 상태를 담을 구조는 아직 없다.
- `Post`는 `Title`, `Content`, `AuthorID`, `BoardID`, 시간 필드만 가지고 있어 `draft`, `soft delete`, `slug`, moderation 상태를 표현하기 어렵다.
- `Comment`는 `ParentID`는 이미 있으나 실제 유스케이스와 요청 모델은 대댓글 입력을 아직 받지 않는다.
- `Comment` 역시 상태 필드와 삭제 시각이 없어 soft delete, 신고 처리, moderation 확장에 불리하다.
- `Notification`과 `PointHistory`는 이벤트 발행 구조가 없는 현재 구조에서는 이르게 도입하면 서비스 간 결합이 커질 가능성이 높다.
- `Attachment`와 `Tag`는 상대적으로 독립성이 높지만, 결국 `Post` 또는 `Comment`의 상태 모델과 결합될 가능성이 있다.

결론

- 신규 도메인을 바로 늘리기보다, 먼저 기존 코어 엔티티를 보강한다.
- 우선 보강 대상은 `User`, `Post`, `Comment`다.
- `User`는 이메일 인증, 비밀번호 재설정, 정지 정책을 수용할 수 있도록 생명주기 모델을 확장한다.
- `Post`는 `draft`, `soft delete`, `slug`, moderation 확장을 고려한 상태 모델을 추가하는 방향이 적절하다.
- `Comment`는 대댓글 지원을 실제 유스케이스/API까지 연결하고, `updated_at`, `deleted_at` 또는 상태 필드를 도입하는 쪽이 적절하다.
- 위 보강 이후 첫 신규 도메인은 `Attachment`, 다음은 `Tag` 순서가 적절하다.
- `Report`는 moderation 상태 모델이 준비된 뒤 도입한다.
- `Notification`, `PointHistory`는 이벤트 버스 또는 이에 준하는 이벤트 경계가 정리된 뒤 도입한다.

권장 도입 순서

1. `User` 생명주기 확장
2. `Post` 상태 모델 보강
3. `Comment` 상태 모델 보강 및 대댓글 경로 연결
4. `Attachment`
5. `Tag`
6. `Report`
7. `Notification`
8. `PointHistory`

후속 작업

- `User` 생명주기 확장 시 필요한 엔티티/포트 초안 정리
- `Post` 상태 모델 초안 정리
- `Comment` 대댓글 API 및 상태 모델 초안 정리
- 이후 `Attachment` 도메인 스코프 정의

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/ARCHITECTURE.md`
- `internal/domain/entity/user.go`
- `internal/domain/entity/post.go`
- `internal/domain/entity/comment.go`
- `internal/application/service/postService.go`
- `internal/application/service/commentService.go`
- `internal/delivery/http_requests.go`

## 2026-03-08 - User 생명주기 확장 범위 1차 결정

상태

- decided

배경

- User 생명주기 확장을 시작하기 전에, 현재 레포 단계에서 무엇을 먼저 넣는 것이 적절한지 정리가 필요했다.
- 이미 사용자 탈퇴와 soft delete는 구현되어 있으므로, 남은 필수 축의 우선순위를 다시 확인했다.

관찰

- 현재 레포는 탈퇴 처리와 soft delete + 익명화 정책을 이미 지원한다.
- 비밀번호 재설정은 이메일 식별/복구 정책이 아직 정해지지 않아 바로 구현하기에는 경계가 크다.
- 운영 관점에서는 계정 복구보다 어뷰징 대응이 먼저 필요할 가능성이 높다.

결론

- 다음 User 생명주기 확장 범위는 비밀번호 재설정이 아니라 관리자 제재 기능이다.
- `User`에 `suspended` 상태를 추가한다.
- 제재 메타데이터는 최소 범위로 `reason string`, `suspended_until`을 둔다.
- 제재 기간은 `7일`, `15일`, `30일`, `무기한`만 허용한다.
- 제재된 사용자는 `post`, `comment` 쓰기 작업을 수행할 수 없도록 정책을 추가한다.
- 비밀번호 재설정은 이메일/복구 정책 결정 후 별도 논의로 미룬다.

후속 작업

- `User` 엔티티와 정책에 suspension 개념 추가
- 관리자용 suspend/unsuspend 유스케이스 및 HTTP 엔드포인트 추가
- `post`, `comment` 생성/수정/삭제 경로에 쓰기 차단 적용

관련 문서/코드

- `docs/ROADMAP.md`
- `internal/domain/entity/user.go`
- `internal/application/policy/authorization_policy.go`
- `internal/application/service/postService.go`
- `internal/application/service/commentService.go`

## 2026-03-08 - 다음 우선순위는 admin 운영 API 보강

상태

- decided

배경

- 사용자 제재 기능은 추가됐지만, 운영자가 현재 제재 상태를 조회하는 읽기 API는 아직 없었다.
- 다음 작업 후보로 `admin API 보강`과 `post/comment 상태 모델 확장` 중 우선순위 판단이 필요했다.

관찰

- 이미 `suspension` 쓰기 정책과 admin 설정/해제 API는 존재한다.
- 반면 운영자가 제재 상태를 확인할 방법이 부족해 기능이 반쯤 열린 상태였다.
- `post/comment` 상태 모델 확장은 중요하지만, 현재 즉시 막히는 운영 흐름은 아니었다.

결론

- 다음 작업은 `post/comment` 상태 모델 확장보다 `admin 운영 API 보강`을 우선한다.
- 1차 보강 범위는 `admin user suspension 조회 API`다.
- 관리자 인증 후 특정 사용자의 현재 상태, 제재 사유, 종료 시각을 조회할 수 있어야 한다.

후속 작업

- `GET /users/{userID}/suspension` 구현
- Swagger / API 문서 반영
- 이후 제재 유저 목록 조회 API 검토

관련 문서/코드

- `internal/delivery/http.go`
- `internal/application/service/userService.go`
- `docs/API.md`

## 2026-03-08 - Post 상태 모델은 기본 3단계로 시작

상태

- decided

배경

- admin 운영 API 보강 이후, 다음 우선순위로 `post/comment` 상태 모델 확장이 남아 있었다.
- 우선 `Post` 상태를 최소 규칙으로 도입해 이후 soft delete와 draft 확장의 기반을 만들 필요가 있었다.

결론

- `Post` 상태는 우선 `draft`, `published`, `deleted` 3단계로 시작한다.
- 현재 기본 글 작성 API는 임시저장 기능이 없으므로 생성 시 기본 상태는 `published`다.
- 삭제는 hard delete가 아니라 `deleted` 상태로 전환하는 soft delete 방식으로 처리한다.
- 공개 목록/상세 조회에서는 `published`만 노출하고 `deleted`는 제외한다.

후속 작업

- `Post` 엔티티에 상태/삭제 시각 추가
- 저장소 조회 규칙을 `published` 중심으로 정리
- 이후 `draft` 전용 API는 별도 작업으로 확장

관련 문서/코드

- `internal/domain/entity/post.go`
- `internal/application/service/postService.go`
- `internal/infrastructure/persistence/inmemory/postRepository.go`

## 2026-03-08 - Draft는 별도 API로 다룬다

상태

- decided

배경

- `Post` 상태에 `draft`가 추가되었으므로, 임시저장과 발행 흐름을 어떤 API로 열지 정할 필요가 있었다.

결론

- 일반 글 작성 API는 기존처럼 생성 즉시 `published`로 둔다.
- draft는 별도 임시저장 API로 생성한다.
- draft 발행은 별도 publish API로 `draft -> published` 상태 전이만 수행한다.

후속 작업

- draft 생성 API 추가
- draft publish API 추가

관련 문서/코드

- `internal/application/service/postService.go`
- `internal/delivery/http.go`

## 2026-03-08 - 삭제/공개 조회 일관성과 suspension 식별자 계약 정리

상태

- decided

배경

- 공개 조회와 soft delete 규칙이 일부 경로에서 일관되지 않았다.
- 게시판 삭제 시 하위 게시글이 남아 aggregate 경계가 흐려질 수 있었다.
- user suspension API는 외부 식별자 정책과 다르게 내부 `user_id`를 노출하고 있었다.

관찰

- 삭제된 게시글 이후에도 댓글/리액션 조회가 계속 가능한 경로가 있었다.
- 게시글 삭제 시 첨부가 orphan 처리되지 않아 cleanup job 대상에서 빠질 수 있었다.
- 게시판 삭제는 현재 hard delete이며, 비어 있지 않은 게시판도 삭제 가능했다.
- 아키텍처 문서는 외부 사용자 식별자로 `uuid` 사용을 원칙으로 둔다.

결론

- 공개 조회는 부모 리소스의 공개 상태를 먼저 확인해야 한다.
- 삭제된 게시글의 댓글/리액션은 공개 조회에서 더 이상 접근할 수 없어야 한다.
- 게시글 삭제 시 하위 댓글은 soft delete 처리하고, 첨부는 orphan 처리해 후속 cleanup 대상으로 넘긴다.
- 게시판 삭제는 비어 있는 게시판에만 허용한다.
- suspension API는 외부 계약을 `user_uuid` 기준으로 정리한다.

후속 작업

- 댓글/리액션 조회 서비스에 부모 공개 상태 검증 추가
- 게시글 삭제 시 댓글/첨부 정리 로직 추가
- `BoardRepository`/`PostRepository` 계약에 게시판 비어 있음 검증을 위한 최소 확장 추가
- suspension API 요청/응답/문서/테스트를 `uuid` 기준으로 정리

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `docs/API.md`
- `internal/application/service/postService.go`
- `internal/application/service/commentService.go`
- `internal/application/service/reactionService.go`
- `internal/application/service/userService.go`
- `internal/delivery/http.go`

## 2026-03-08 - Comment 상태 모델도 soft delete 기준으로 맞춘다

상태

- decided

배경

- `Post`는 `deleted` 상태 기반 soft delete로 전환됐지만 `Comment`는 아직 hard delete 성격으로 남아 있었다.

결론

- `Comment`는 우선 `active`, `deleted` 2단계 상태로 시작한다.
- `updated_at`, `deleted_at`을 추가한다.
- 삭제는 hard delete가 아니라 `deleted` 상태로 전환하는 soft delete 방식으로 처리한다.
- 공개 조회에서는 `deleted` 댓글을 제외한다.

후속 작업

- `Comment` 엔티티/저장소/서비스 soft delete 반영
- 이후 대댓글 API 연결 시 같은 상태 규칙을 따른다

관련 문서/코드

- `internal/domain/entity/comment.go`
- `internal/application/service/commentService.go`
- `internal/infrastructure/persistence/inmemory/commentRepository.go`

## 2026-03-08 - 대댓글은 parentID 저장, 정책은 1-depth 제한

상태

- decided

배경

- `Comment.ParentID`는 이미 있었지만 실제 생성 API와 서비스 규칙에는 아직 연결되지 않았다.

결론

- 저장 모델은 계속 `parentID`를 사용한다.
- 현재 서비스 정책은 1-depth 대댓글만 허용한다.
- 부모 댓글은 같은 게시글에 속한 활성 댓글이어야 한다.
- 조회는 기존 flat list + `parent_id` 노출을 유지하고, 최적화는 어댑터 책임으로 둔다.

후속 작업

- comment create API에 `parent_id` 연결
- 1-depth 검증 로직 추가

관련 문서/코드

- `internal/domain/entity/comment.go`
- `internal/application/service/commentService.go`
- `internal/delivery/http_requests.go`

## 2026-03-08 - Attachment는 Post 전용 메타데이터부터 시작

상태

- decided

배경

- 코어 상태 모델과 draft/reply 흐름이 정리된 뒤, 다음 확장 도메인으로 `Attachment`를 도입할 시점이 되었다.

결론

- `Attachment`는 우선 `Post` 전용 메타데이터 도메인으로 시작한다.
- 현재 단계에서는 파일 업로드 자체가 아니라 메타데이터만 다룬다.
- 최소 필드는 `file_name`, `content_type`, `size_bytes`, `storage_key`다.
- 공개 조회는 published post 기준으로만 허용한다.
- 작성자 또는 admin은 draft/published post에 attachment를 추가/삭제할 수 있다.

후속 작업

- attachment entity/repository/usecase/service 추가
- post attachment 생성/조회/삭제 API 추가
- post detail 포함 여부는 2차 확장으로 미룬다

관련 문서/코드

- `internal/application/service/postService.go`
- `internal/delivery/http.go`
- `docs/ROADMAP.md`

## 2026-03-08 - 실제 파일 저장은 draft first + FileStorage 포트로 준비

상태

- decided

배경

- attachment 메타데이터는 추가됐지만, 실제 이미지 파일 저장 경로와 저장 어댑터 경계는 아직 없었다.

결론

- 현재 업로드 흐름은 `draft post`에 먼저 연결하는 방향을 기준으로 잡는다.
- 미래에는 post 없이 선업로드 후 나중에 bind/event로 연결할 수 있도록 파일 저장은 별도 `FileStorage` 포트로 분리한다.
- 1차 구현체는 local filesystem 어댑터로 시작한다.
- 나중에 object storage 어댑터를 같은 포트 구현으로 추가한다.

후속 작업

- `FileStorage` 포트 추가
- local filesystem 어댑터 추가
- storage root config 추가
- draft post attachment upload API 추가

관련 문서/코드

- `internal/application/port`
- `internal/infrastructure`
- `internal/config/config.go`

## 2026-03-08 - Post attachment 업로드는 FileStorage + Attachment 메타를 함께 쓴다

상태

- decided

배경

- 실제 이미지 업로드를 열기 위해서는 파일 바이트 저장과 post 연결 메타를 분리해 다룰 필요가 있었다.

결론

- 현재 업로드는 `draft/published post`에 대해 바로 수행한다.
- 파일 본문은 `FileStorage`가 저장하고, post 연결과 메타데이터는 `AttachmentRepository`가 관리한다.
- upload API는 multipart file 입력을 받아 `storage_key`를 생성하고 attachment 메타를 함께 기록한다.

후속 작업

- upload API 문서/Swagger 반영
- 나중에 post 없는 선업로드 + bind 흐름 검토

관련 문서/코드

- `internal/application/service/attachmentService.go`
- `internal/application/port/file_storage.go`
- `internal/application/port/attachment_repository.go`

## 2026-03-08 - 본문 내 이미지는 attachment 참조를 직접 심는 방식으로 간다

상태

- decided

배경

- post에 attachment를 연결할 수 있어도, 본문 중간 삽입 순서와 노출 위치를 별도 attachment 목록만으로는 보장할 수 없었다.

결론

- 본문은 Markdown 이미지 문법으로 `attachment://{attachmentID}` 참조를 직접 가진다.
- upload API는 본문에 바로 넣을 수 있는 `embed_markdown`을 응답한다.
- `PostDetail`은 attachment 목록을 함께 내려 클라이언트가 본문 참조를 해석할 수 있게 한다.
- attachment 응답에는 실제 파일 조회용 `file_url`을 포함한다.
- 1차 파일 조회는 published post 기준 public read 경로로 연다.
- draft 작성 중 미리보기는 owner/admin 전용 authenticated `preview_url`로 제공한다.
- 파일 캐시는 앱 메모리 캐시보다 HTTP 캐시 헤더를 우선 적용한다.
- `file_url`은 `Cache-Control: public` + `ETag`를 사용하고, `preview_url`은 `private, no-store`로 둔다.
- attachment 업로드는 우선 이미지 화이트리스트(`png`, `jpeg/jpg`, `gif`, `webp`)와 설정 가능한 최대 크기 제한을 둔다.
- 업로드 MIME은 요청 헤더만 믿지 않고 본문 sniffing 결과와 일치해야 한다.
- storage key는 같은 파일명 충돌을 피하기 위해 `postID/랜덤-sanitized-name` 규칙으로 생성한다.
- `storage_key`는 내부 저장 메타데이터로만 유지하고 외부 응답에서는 숨긴다.
- 외부 `manual metadata create API`는 제거하고 attachment 생성은 upload 기반으로만 연다.
- post update/publish 시 본문에 들어 있는 attachment 참조가 실제로 해당 post의 attachment인지 검증한다.

후속 작업

- 파일 다운로드/노출 경로 설계
- 필요 시 markdown renderer 또는 frontend 해석 규칙 정리

관련 문서/코드

- `internal/application/service/postService.go`
- `internal/application/service/attachmentService.go`
- `docs/API.md`

## 2026-03-08 - Attachment 후속은 orphan 표시, object storage adapter, 서버 내부 이미지 최적화로 간다

상태

- decided

배경

- attachment 업로드/참조/조회 흐름은 갖춰졌지만, 미사용 파일 정리 정책과 저장소 확장, 저장 효율화는 아직 남아 있었다.

결론

- unused attachment는 즉시 삭제하지 않고 `Attachment`에 orphan 표시를 남긴다.
- orphan attachment는 public 응답/공개 파일 조회에서는 제외하고, owner/admin preview에서는 유지한다.
- 실제 삭제는 나중 배치 잡이 처리할 수 있도록 지연 정리 정책을 전제로 한다.
- 파일 저장 포트 구현체로 S3-compatible object storage adapter를 추가하되, 기본 provider는 계속 local로 둔다.
- 이미지 후처리는 업로드 시점에 서버 내부 저장본만 최적화한다.
- 1차 최적화 범위는 `jpeg/jpg`, `png` 재인코딩이다. `gif`, `webp`는 원본 유지로 시작한다.

후속 작업

- attachment orphan 필드/표시 정책 구현
- storage provider config와 object storage adapter 추가
- 업로드 시 이미지 최적화 처리 추가

관련 문서/코드

- `internal/domain/entity/attachment.go`
- `internal/application/service/attachmentService.go`
- `internal/application/service/postService.go`
- `internal/infrastructure/storage`

## 2026-03-08 - 주기 작업은 공통 job runner 위에 올리고 orphan cleanup은 첫 작업으로 넣는다

상태

- decided

배경

- orphan attachment는 표시만 있고 실제 정리 경로는 아직 없다.
- 하지만 orphan cleanup 하나만을 위한 ad-hoc ticker를 `main.go`에 직접 넣는 방식은 이후 배치 작업 확장에 불리하다.

결론

- 주기 작업은 공통 in-process job runner 위에 등록하는 방식으로 시작한다.
- 현재 1차 작업은 `orphan attachment cleanup`이며, 이후 다른 정리 작업도 같은 runner에 추가할 수 있게 한다.
- 작업별 on/off, 주기, grace period, batch size는 config로 관리한다.
- orphan cleanup은 `orphaned_at + grace period`가 지난 attachment만 정리한다.
- cleanup은 저장 파일 삭제 후 attachment 메타데이터 삭제까지 수행한다.
- 변경 작업의 기본 순서는 `결정 문서 기록 -> TDD -> 구현 -> 테스트 통과 -> 문서 정합성 반영 -> 커밋/푸시`로 고정한다.

후속 작업

- 공통 job runner 추가
- orphan cleanup 유스케이스 구현
- jobs config 및 서버 시작 시 runner 등록

관련 문서/코드

- `cmd/main.go`
- `internal/application/service/attachmentService.go`
- `internal/infrastructure/job`
- `internal/config/config.go`

## 2026-03-08 - Attachment 후속 구현 상태를 코드 기준으로 완료 처리한다

상태

- decided

배경

- `docs/DECISIONS.md`와 `docs/ROADMAP.md`에는 Attachment 후속 작업이 아직 남아 있는 듯한 표현이 남아 있었다.
- 실제 코드 기준으로 orphan 정책, storage adapter, 이미지 최적화, cleanup job runner가 어디까지 반영됐는지 다시 확인할 필요가 있었다.

관찰

- `Attachment` 엔티티는 `orphaned_at`, `pending_delete_at`에 해당하는 상태를 이미 가진다.
- `AttachmentService`는 공개 조회/공개 파일 조회에서 orphan 및 `pending_delete` 첨부를 숨기고, owner/admin preview에서는 orphan는 허용하고 `pending_delete`는 차단한다.
- 업로드 시 `jpeg/png` 최적화가 설정값으로 제어되며, 허용 타입/용량/sniffing 검증도 반영돼 있다.
- `FileStorage`는 `local`과 `object` provider를 모두 지원하며, `cmd/main.go`에서 설정 기반으로 조립된다.
- 공통 in-process job runner가 존재하고, 서버 부팅 시 attachment cleanup job이 config 기반으로 등록된다.
- attachment cleanup 유스케이스는 orphan 및 `pending_delete` 대상을 grace period 이후 실제 파일 삭제 + 메타데이터 삭제까지 수행한다.

결론

- Attachment 후속으로 결정했던 아래 항목은 현재 코드 기준 이미 반영된 것으로 본다.
  - orphan 표시/노출 정책
  - object storage adapter
  - 서버 내부 이미지 최적화
  - 공통 in-process job runner
  - orphan attachment cleanup 유스케이스
  - jobs config 및 서버 시작 시 runner 등록
- 따라서 로드맵의 현재 상태 메모는 Attachment 후속과 background job 기반 cleanup까지 반영된 상태로 갱신한다.
- Step 2에서 Attachment 다음 확장 도메인 우선순위는 기존 결정대로 `Tag -> Report -> Notification/PointHistory` 순서를 유지한다.

후속 작업

- `Tag` 도메인 스코프와 API 초안 정리
- `Report` 도메인 착수 전 moderation 상태 모델 보강 범위 재점검
- Attachment cleanup 실행 결과를 관측할 수 있도록 observability 보강 시 job 로그/메트릭 정리

관련 문서/코드

- `docs/ROADMAP.md`
- `cmd/main.go`
- `internal/domain/entity/attachment.go`
- `internal/application/service/attachmentService.go`
- `internal/infrastructure/job/inprocess/runner.go`
- `internal/infrastructure/storage/localfs/fileStorage.go`
- `internal/infrastructure/storage/object/fileStorage.go`
- `internal/config/config.go`


## 2026-03-09 - Tag 도메인은 Post 상세 확장 + 다중 저장소 쓰기 단위로 도입한다

상태

- decided

배경

- `Tag` 도메인을 `Post`와 N:M 관계로 도입하려고 한다.
- 요구사항에는 태그 생성/재사용, 관계 soft delete 복구, post 삭제 시 관계 비활성화, post 상세 응답 포함, tag 클릭 기반 post 조회가 포함된다.
- 현재 레포는 `PostService`가 `Post`, `Attachment`, `Comment`, `Reaction` 저장소를 직접 조합하지만, 여러 도메인 쓰기를 하나의 작업 단위로 묶는 추상화는 없다.
- 현재 공개 API/DTO/Swagger에는 `tags` 입력 및 응답 계약이 없다.

관찰

- `postRequest`는 현재 `title`, `content`만 받으며 태그 입력을 표현하지 못한다.
- `PostUseCase`, `PostDetail`, `response.PostDetail`에는 태그 개념이 없다.
- 현재 post 상세 캐시는 태그 없는 완성형 DTO를 저장한다.
- `PostRepository`만으로는 `Tag`, `PostTag` 생성/재사용/복구/비활성화 규칙을 표현하기 어렵다.
- 현재 코드베이스는 `PostStatus`, `ReactionType`처럼 문자열 상태를 typed enum 상수로 관리하는 패턴을 사용한다.

결론

- `Tag`는 `Post`와 별도 도메인으로 두고, 연결은 `PostTag` 조인 도메인으로 관리한다.
- `Tag`는 `id`, 정규화된 `name`, `created_at`을 가진다.
- `Tag.name`은 저장 전에 `trim + lowercase`로 정규화하고, 정규화된 값 기준으로 애플리케이션과 저장소에서 모두 유일성을 보장한다.
- `PostTag.status`는 raw string이 아니라 typed enum 상수(`active`, `deleted`)로 관리한다.
- post 생성/수정 요청은 `tags[]`를 받을 수 있어야 한다.
- `tags[]`는 최대 개수와 각 태그 최대 길이를 검증하고, 앞뒤 공백 제거, 소문자 정규화, 중복 제거 후 처리한다.
- post 상세 응답과 상세 캐시는 `tags`를 포함한 완성형 DTO를 기준으로 한다.
- post 목록 응답에는 태그를 포함하지 않는다.
- post 수정 시 기존 관계는 diff 기반으로 처리한다.
- 새 태그는 연결하고, 제거된 태그 관계는 `deleted`로 표시하며, 기존 `deleted` 관계가 다시 들어오면 `active`로 복구한다.
- post 삭제 시 tag 자체는 삭제하지 않고 관련 `PostTag`만 `deleted`로 전환한다.
- tag 클릭 use case는 태그 기준 post 목록 조회 API로 별도 제공한다.
- 여러 저장소를 함께 갱신하는 post/tag 쓰기 유스케이스를 위해 저장소 구현과 분리된 `UnitOfWork`(또는 동등한 transaction manager) 포트를 도입한다.
- 이 추상화는 RDB 전용 개념이 아니라, 다중 도메인 쓰기를 하나의 커밋/롤백 단위로 다루기 위한 application-level 경계로 본다.

후속 작업

- `Tag`, `PostTag` 엔티티 및 저장소 포트 추가
- `UnitOfWork` 포트와 in-memory 구현 초안 추가
- post create/update/delete 서비스에 tag 동기화 로직 연결
- post detail DTO/응답/캐시/Swagger에 `tags` 반영
- tag 기반 post 목록 조회 API 추가
- tag 정규화/유니크/복구 규칙에 대한 계약 테스트 추가

관련 문서/코드

- `docs/API.md`
- `docs/ARCHITECTURE.md`
- `internal/application/service/postService.go`
- `internal/application/port/post_usecase.go`
- `internal/application/model/post_detail.go`
- `internal/delivery/http_requests.go`
- `internal/delivery/response/types.go`
- `internal/application/cache/key/keys.go`

## 2026-03-09 - Tag 기반 공개 조회는 PostRepository 책임으로 두고 In-Memory 저장소는 clone 반환을 원칙으로 한다

상태

- decided

배경

- tag 기반 post 조회를 어느 포트 책임으로 둘지와, in-memory 저장소가 내부 엔티티 포인터를 직접 반환하는 현재 패턴을 유지할지 판단이 필요했다.
- `UnitOfWork`를 tx-bound repository 방식으로 바꾸면서 저장소 반환값과 서비스의 엔티티 변경 방식도 함께 정리할 필요가 있었다.

관찰

- `published posts by tag`는 입력은 tag지만, 실제 정책은 post 공개 여부와 pagination 규칙에 좌우된다.
- `PostTagRepository`에 공개 조회 정책까지 밀어 넣으면 relation 생명주기와 post 공개 정책이 섞인다.
- 현재 in-memory 저장소는 내부 포인터를 그대로 반환하고, 서비스가 lock 밖에서 엔티티를 직접 mutate하는 경로가 존재한다.

결론

- 현재 단계에서는 `published posts by tag`를 별도 query port로 분리하지 않고 `PostRepository` 책임으로 둔다.
- 레포 규모가 커지면 이후 `PostQueryRepository` 같은 읽기 전용 포트로 확장할 수 있다.
- in-memory 저장소는 전체 저장소(`User/Board/Post/Tag/PostTag/Comment/Reaction/Attachment`)에서 조회 시 clone을 반환하고, 서비스는 copy-on-write 방식으로 엔티티를 갱신한다.
- 저장소 내부 객체를 직접 외부에 노출하는 패턴은 지양한다.
- `UnitOfWork`는 특정 저장소 구현 세부가 아니라 애플리케이션의 명시적 tx 경계 포트로 유지한다.
- 각 어댑터는 해당 포트를 자기 방식으로 구현한다.
  - in-memory: tx-bound repository + snapshot rollback + tx 중 외부 접근 차단
  - RDB: 실제 DB transaction + tx-bound repository
- write use case는 `조회 -> 검증 -> 갱신`을 하나의 tx 안에서 처리하고, 캐시 삭제 호출만 tx 밖에서 best effort로 수행한다.
- 캐시 무효화에 필요한 식별자 목록도 tx 안에서 확정한 뒤 밖으로 전달한다.

후속 작업

- `PostRepository.SelectPublishedPostsByTagName(...)` 추가
- tag 조회 서비스가 해당 포트를 사용하도록 정리
- in-memory 저장소 clone 반환 패턴을 전체 저장소로 확장
- write service 전반을 `조회 -> 검증 -> 갱신` tx 패턴으로 정리

관련 문서/코드

- `internal/application/port/post_repository.go`
- `internal/application/service/postService.go`
- `internal/infrastructure/persistence/inmemory/postRepository.go`
- `internal/application/service/commentService.go`
- `internal/application/service/attachmentService.go`

## 2026-03-10 - bootstrap admin과 인증 비밀값은 명시적 설정으로만 연다

상태

- decided

배경

- 현재 구성 루트는 서버 시작 시 고정 `admin/admin` 계정을 자동 시드한다.
- 기본 `config.yml`에는 고정 JWT secret 값이 들어 있어, 설정 파일을 그대로 사용할 경우 보안상 취약하다.
- 계정 삭제 후 세션 정리 실패를 허용하는 현재 정책은 deleted user가 기존 토큰으로 잠시 재인증될 여지를 남긴다.

관찰

- 학습용/로컬 편의 기능과 배포 가능한 기본 동작이 같은 경로에 섞여 있다.
- 인증 경계는 현재 `JWT 유효성 + 세션 캐시 존재`만 확인하고 사용자 생명주기를 다시 확인하지 않는다.
- 로그인 핸들러는 다른 JSON 엔드포인트와 다르게 transport validation을 건너뛴다.

결론

- admin bootstrap은 기본 비활성화하고, config에서 명시적으로 `enabled` 했을 때만 수행한다.
- bootstrap admin의 `username`, `password`는 config에서 받되, 빈 값과 알려진 기본값(`admin`) 같은 placeholder는 허용하지 않는다.
- JWT secret도 committed default를 두지 않고, placeholder/빈 값이면 시작 실패로 처리한다.
- 토큰 검증 시 세션 캐시뿐 아니라 사용자 존재/삭제 상태도 다시 확인해 deleted user의 stale session을 차단한다.
- 로그인 요청도 signup과 동일하게 request validation을 먼저 수행한다.

후속 작업

- config에 bootstrap admin 섹션 추가 및 검증 규칙 반영
- `cmd/main.go`의 unconditional seed 제거
- `SessionService` 토큰 검증에 사용자 상태 확인 추가
- login handler 요청 검증 테스트/구현 추가
- 운영 문서에서 secret/bootstrap 설정법 갱신

관련 문서/코드

- `cmd/main.go`
- `config.yml`
- `docs/ARCHITECTURE.md`
- `docs/CONFIG.md`
- `internal/application/service/sessionService.go`
- `internal/delivery/http.go`

## 2026-03-10 - upload 제한은 HTTP와 service 양쪽에서 강제하고 config는 env-only도 허용한다

상태

- decided

배경

- attachment 업로드는 서비스 레이어에서 파일 크기를 검사하지만, multipart 파싱 이전의 HTTP 경계 제한은 아직 없었다.
- 설정 문서는 환경 변수 기반 구성을 안내하지만, 실제 로더는 config 파일이 없으면 실패했다.
- application 서비스는 운영 로그를 남기기 위해 전역 `slog`에 직접 의존하고 있었다.

관찰

- 업로드 한도는 resource control 성격이 강해서 delivery와 service 둘 다에서 막아야 한다.
- env-only 실행은 컨테이너/배포 환경에서 유용하며, 현재 문서도 그 방향을 암묵적으로 기대한다.
- 로깅은 필요하지만 application이 concrete logger를 직접 알 필요는 없다.

결론

- attachment 업로드 한도는 `HTTP body/multipart 파싱 전 제한 + service stream 제한`의 이중 경계로 적용한다.
- `Config.Load()`는 config 파일이 없어도 환경 변수와 default만으로 로드 가능해야 한다.
- application 서비스는 전역 logger 대신 logger port에만 의존한다.
- composition root가 실제 logger adapter를 주입하고, 테스트/기본 경로는 noop logger를 사용한다.

후속 작업

- HTTP server/dependencies에 upload max bytes 설정 연결
- config loader에서 file-not-found를 non-fatal 처리하고 회귀 테스트 추가
- logger port/noop/slog adapter 추가 및 service 생성자 정리
- docs에서 env-only 설정과 업로드 제한 규칙 명시

관련 문서/코드

- `internal/delivery/http.go`
- `internal/application/service/attachmentService.go`
- `internal/config/config.go`
- `internal/application/service/cache_invalidation.go`
- `internal/application/service/accountService.go`

## 2026-03-10 - delivery 에러 분류는 transport/auth/internal 실패를 구분한다

상태

- decided

배경

- framework-level `404/405`는 현재 올바른 status를 내려도 body는 `internal server error`로 응답된다.
- auth middleware는 invalid token과 repository/cache failure를 모두 `401 Unauthorized`로 처리한다.
- config 검증은 placeholder secret은 거부하지만 공백-only secret은 허용한다.

관찰

- transport miss와 use case/internal failure는 같은 에러 분류가 아니다.
- 인증 실패와 인증 경계 내부 장애를 같은 status로 숨기면 운영 문제를 진단하기 어려워진다.
- secret 검증은 trim 후 빈 값도 거부해야 bootstrap credential 검증과 같은 강도를 유지한다.

결론

- `NoRoute`, `NoMethod`는 public sentinel error를 사용해 status/body/log 의미를 일치시킨다.
- auth middleware는 `missing/invalid token -> 401`, `repository/cache failure -> 500`으로 구분한다.
- JWT secret 검증은 `strings.TrimSpace(...)` 기준으로 수행한다.

후속 작업

- delivery 테스트에 `404/405` body 검증 추가
- auth middleware 테스트에 infra failure -> `500` 분기 추가
- config 테스트에 whitespace-only secret 거부 케이스 추가

관련 문서/코드

- `internal/delivery/http.go`
- `internal/delivery/middleware/authMiddleware.go`
- `internal/application/service/sessionService.go`
- `internal/config/config.go`

## 2026-03-10 - embed 출력과 background job config, delivery logger 경계를 실제 구현과 맞춘다

상태

- decided

배경

- attachment upload 응답은 `embed_markdown`에 raw filename을 그대로 넣어 Markdown 문법을 깨뜨릴 수 있다.
- config loader는 `jobs.enabled=false`여도 cleanup job 세부 설정을 계속 강제한다.
- 아키텍처 문서는 delivery 로깅이 composition root에서 주입된 logger를 쓴다고 설명하지만, 실제 구현은 아직 전역 `slog`에 직접 의존한다.

관찰

- user-controlled filename을 다른 문법으로 재직렬화할 때는 출력 escaping 규칙이 필요하다.
- feature flag로 완전히 비활성화된 기능의 세부 설정까지 강제하면 runtime contract와 validation contract가 어긋난다.
- logger port migration은 application까지만 끝난 상태라 delivery 경계가 문서와 구현 사이에서 어긋난다.

결론

- `embed_markdown`은 raw filename 대신 Markdown-safe alt text를 사용한다.
- `jobs.attachmentCleanup.*` 검증은 `jobs.enabled && jobs.attachmentCleanup.enabled`일 때만 강제한다.
- delivery는 `HTTPDependencies`로 logger를 받아 request failure 로깅에도 같은 경계를 사용한다.

후속 작업

- attachment service 테스트에 Markdown metacharacter filename 케이스 추가
- config 테스트에 jobs disabled 시 relaxed validation 케이스 추가
- delivery 테스트/구성 루트에 logger dependency 연결

관련 문서/코드

- `internal/application/service/attachmentService.go`
- `internal/config/config.go`
- `internal/delivery/http.go`
- `cmd/main.go`
