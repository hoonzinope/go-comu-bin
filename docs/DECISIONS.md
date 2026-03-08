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
