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
