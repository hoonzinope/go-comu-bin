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
