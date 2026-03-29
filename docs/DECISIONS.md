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

## 2026-03-27 - runtime cache는 Ristretto-backed adapter로 전환하고 in-memory는 test double로 남긴다

상태

- decided

배경

- 현재 runtime cache는 직접 구현한 in-memory adapter를 쓰고 있고, 조회 캐시와 세션 캐시가 같은 port를 공유한다.
- cache write/read hot path는 요청 경로와 이벤트 무효화 경로 모두에서 쓰이므로, 더 높은 처리량과 bounded memory behavior가 필요했다.
- prefix 기반 무효화 의미는 그대로 유지해야 해서, 단순 교체가 아니라 prefix index를 보조 구조로 두는 설계가 필요했다.

관찰

- `cmd/main.go`와 integration bootstrap은 현재 `internal/infrastructure/cache/inmemory`를 직접 주입한다.
- `port.Cache`는 `GetOrSetWithTTL`과 prefix 연산을 포함하고 있어, adapter 내부 구현만 바꾸면 상위 서비스 계약은 유지된다.
- Ristretto는 write buffering과 admission policy를 가지므로, write 성공 여부를 명시적으로 확인하고 `Wait()`로 flush해야 즉시 조회 가능성을 보장할 수 있다.

결론

- runtime cache는 `internal/infrastructure/cache/ristretto` 어댑터로 전환한다.
- `DeleteByPrefix`와 `ExistsByPrefix`는 prefix index를 별도로 유지해 정확한 의미를 보장한다.
- cache write가 reject/drop되면 error로 올리고, application은 기존처럼 cache failure로 처리한다.
- cache capacity 관련 설정은 `cache` 설정으로 노출한다.
- 기존 in-memory cache 구현은 테스트 더블과 reference adapter 용도로 유지한다.

후속 작업

- Ristretto adapter 구현 및 runtime bootstrap 전환
- cache 설정 확장 및 validation/test 갱신
- architecture/testing 문서에 runtime/test double 경계 반영

관련 문서/코드

- `cmd/main.go`
- `internal/infrastructure/cache/ristretto`
- `internal/infrastructure/cache/inmemory`
- `internal/config/config.go`
- `docs/ARCHITECTURE.md`

## 2026-03-25 - ranking v1은 전역 feed API + PublishedAt + outbox 기반 비동기 점수 갱신으로 도입한다

상태

- decided

배경

- 현재 공개 post 목록은 board/tag/search 단위로만 제공되고, 전역 소비 피드와 정렬 다양화(`hot`, `best`)는 아직 없다.
- post search는 이미 `score desc + composite cursor` 규약을 사용하고 있어 ranking read-side를 같은 방식으로 확장할 수 있다.
- 현재 구조에는 조회수 개념이 없고, outbox relay 기반 비동기 갱신 경계는 이미 마련돼 있다.

관찰

- `Post`는 draft/published 상태를 가지지만 실제 공개 시점 필드가 없어, 오래된 draft를 publish할 때 `CreatedAt`만으로는 최신성 의미가 어긋난다.
- `post.changed`, `comment.changed`, `reaction.changed` 이벤트는 ranking 계산에 필요한 일부 정보를 아직 충분히 담지 못한다.
- hidden board 필터는 read path에서 이미 적용되고, 공개 목록 응답은 `PostList` shape로 통일돼 있다.

결론

- ranking v1 공개 API는 새 전역 feed endpoint `GET /api/v1/posts/feed`로 제공한다.
- 지원 sort는 `hot`, `best`, `latest` 세 가지로 고정하고, 기본 sort는 `hot`으로 둔다.
- 응답은 기존 `PostList`를 재사용하며 score는 외부에 노출하지 않는다.
- `Post`에는 `PublishedAt`을 추가한다.
  - 일반 post 생성 시 `PublishedAt=CreatedAt`
  - draft 생성 시 `PublishedAt=nil`
  - draft publish 시 `PublishedAt=now`
  - update/delete는 `PublishedAt`을 바꾸지 않는다.
- ranking 시간축은 항상 `PublishedAt`을 사용한다.
- ranking 점수 입력은 v1에서 `reaction + comment + time`만 사용하고 view count는 포함하지 않는다.
- `best`는 최근 7일 내 활동 점수로 계산한다. 글 생성 시점 제한은 두지 않는다.
- 점수 규칙은 다음으로 고정한다.
  - reaction: `like=+1`, `dislike=-1`
  - comment created: `+2`
  - comment deleted: `-2`
- `latest`: `published_at desc, post_id desc`
- `best`: `recent_7d_score desc, published_at desc, post_id desc`
- `hot`: Reddit 스타일 시간 감쇠를 사용한다.
  - `base = total_reaction_score + total_comment_score`
  - `order = log10(max(abs(base), 1))`
  - `sign = -1|0|1`
  - `seconds = publishedAt.Unix() - 1134028003`
  - `hot = sign*order + seconds/45000`
- ranking 갱신은 write path 직접 계산이 아니라 outbox relay 소비로 처리한다.
- 새 read-side 포트 `PostRankingRepository`를 도입하고, in-memory reference adapter를 구현한다.
- `best` 계산용 최근 7일 activity ledger는 lazy prune으로 정리하고 별도 cleanup job은 두지 않는다.
- hidden board, unpublished post, deleted post는 feed 결과에서 제외한다.

후속 작업

- `PublishedAt` 반영과 event payload 확장
- in-memory ranking repository / cursor / handler / feed query 추가
- HTTP/Swagger/API/ARCHITECTURE/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `internal/domain/entity/post.go`
- `internal/application/event/types.go`
- `internal/application/service/post/query_handler.go`

## 2026-03-25 - ranking v2는 공통 sort/window 규약으로 feed/board/tag/search 목록을 확장한다

상태

- decided

배경

- ranking v1은 전역 feed에만 `hot/best/latest`를 제공하고, board/tag/search 목록은 각자 다른 정렬 규약을 유지한다.
- SQLite/영속화 이슈는 후속 단계에서 다루기로 했으므로, 이번 단계는 공개 API 규약과 application read path 확장에 집중한다.

관찰

- feed ranking projection과 outbox relay 갱신 경로는 이미 존재한다.
- board/tag/search는 각각 `ID cursor`, `tag post list`, `BM25 relevance cursor`로 분리돼 있어 정렬 규약이 통일돼 있지 않다.
- 현재 점수 입력은 `reaction + comment + time`만 사용하며, view count는 아직 없다.

결론

- ranking v2는 공개 목록 API 전반에 공통 `sort/window` query 규약을 도입한다.
- 적용 대상:
  - `GET /api/v1/posts/feed`
  - `GET /api/v1/boards/{boardUUID}/posts`
  - `GET /api/v1/tags/{tagName}/posts`
  - `GET /api/v1/posts/search`
- 지원 정렬:
  - `feed`, `board`, `tag`: `hot|best|latest|top`
  - `search`: `relevance|hot|latest|top`
- 기본 정렬:
  - `feed`: `hot`
  - `board`, `tag`: `latest`
  - `search`: `relevance`
- `window`는 `sort=top`일 때만 허용하고, 기본값은 `7d`로 둔다.
- 허용 window는 `24h`, `7d`, `30d`, `all` 네 가지로 고정한다.
- `top`은 기간별 누적 점수 정렬이며, 점수 입력은 v1과 동일하게 유지한다.
  - `like=+1`
  - `dislike=-1`
  - `comment created=+2`
  - `comment deleted=-2`
- `best`는 기존 최근 7일 activity score 규칙을 유지하고, search에는 도입하지 않는다.
- ranking cursor는 sort와 window를 모두 포함하는 opaque cursor로 확장한다.
- search 기본 `relevance`는 기존 BM25 규칙을 유지하되, cursor도 `sort/window`를 포함하는 opaque 형식으로 맞춘다.
- score와 applied window 같은 메타데이터는 외부 응답에 노출하지 않는다.
- board/tag/search의 ranking 정렬은 현재 ranking read-side를 재사용하고, search의 ranking 정렬은 검색 결과 집합 내부 재정렬로 해석한다.
- SQLite 영속화, projection rebuild admin/job, 운영 메트릭은 이번 범위에서 제외한다.

후속 작업

- ranking read-side에 `top`/window 집계 규약 추가
- board/tag/search query에 공통 sort/window validation + cursor 규약 반영
- search relevance cursor 확장 및 ranking 재정렬 경로 추가
- HTTP/Swagger/API/ARCHITECTURE/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `internal/application/port/post_ranking_repository.go`
- `internal/application/service/post/query_handler.go`

## 2026-03-25 - password reset v2는 FE 링크 메일 + IP/email rate limit + cleanup job/audit log로 강화한다

상태

- decided

배경

- password reset v1은 token 발급/확인과 전체 세션 무효화까지는 닫았지만, 실제 메일 UX는 raw token 전달 수준에 머물러 있다.
- 현재 `/api/v1` 전역 rate limit만으로는 password reset request 엔드포인트의 타깃성 abuse를 별도로 제어하기 어렵다.
- reset token 저장소에는 만료/소비 토큰 정리와 운영 추적을 위한 cleanup/audit 규약이 아직 없다.

관찰

- SMTP sender는 현재 token 평문을 메일 본문에 직접 넣는다.
- HTTP는 이미 공통 `RateLimiter` 포트와 `429 Too Many Requests` 공개 에러 규약을 사용한다.
- background job runner는 attachment/guest cleanup에 이미 사용 중이며, 같은 패턴으로 token cleanup을 등록할 수 있다.
- account service는 `slog` logger를 보유하고 있어 별도 audit 저장소 없이 구조화 로그를 남길 수 있다.

결론

- password reset 메일은 frontend reset 페이지 링크를 사용한다.
- 새 설정값 `delivery.mail.passwordReset.baseURL`을 도입하고, 링크 형식은 `${baseURL}?token=<urlqueryescaped token>`으로 고정한다.
- 메일 본문에는 reset 링크, 만료 시각, fallback raw token을 함께 포함한다.
- `POST /api/v1/auth/password-reset/request`에는 전용 rate limit을 추가한다.
- 전용 key는 `password-reset-request:<client-ip>:<normalized-email>`로 고정하고, 존재하지 않는 email도 동일하게 카운트한다.
- 제한 초과 시 공개 응답은 `429 Too Many Requests`를 사용한다.
- password reset token 저장소에는 cleanup 연산을 추가하고, cleanup 대상은 다음으로 고정한다.
  - `ConsumedAt != nil && ConsumedAt <= now - gracePeriod`
  - `ExpiresAt <= now - gracePeriod`
- cleanup은 background job `password-reset-cleanup`으로 수행한다.
- structured audit log는 account service에서 남기고, raw token/plain email/new password는 로그에 남기지 않는다.
- 별도 audit 저장소와 email verification cleanup은 이번 범위에서 제외한다.

후속 작업

- config, SMTP sender, HTTP handler, token repository, background job wiring 반영
- account service audit log 및 cleanup use case 추가
- 테스트, Swagger, API/CONFIG/ARCHITECTURE/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/CONFIG.md`
- `internal/application/service/account/service.go`
- `internal/infrastructure/mail/smtp/sender.go`

## 2026-03-25 - account security hardening은 login/guest-upgrade 전용 rate limit과 auth audit log로 강화한다

상태

- decided

배경

- 현재 `/api/v1` 전역 rate limit만으로는 login brute-force나 guest upgrade 타깃성 abuse를 엔드포인트별로 충분히 제어하기 어렵다.
- auth 민감 경로에는 성공/실패 이유를 구조적으로 남기는 audit log가 부족해 운영 추적성이 약하다.
- lockout, captcha, MFA까지 한 번에 도입하기에는 상태 관리와 제품 정책 결정이 커져 현재 범위를 넘는다.

관찰

- login은 `SessionService.Login`에서 credential 검증과 세션 저장을 처리한다.
- guest upgrade는 `AccountService.UpgradeGuestAccount`가 세션 교체와 사용자 승격을 하나의 성공 경계로 관리한다.
- password reset / email verification v2에서 HTTP handler 전용 rate limit과 service audit log 패턴을 이미 도입했다.

결론

- `POST /api/v1/auth/login`에 전용 rate limit을 추가한다.
  - key: `login:<client-ip>:<normalized-username>`
- `POST /api/v1/auth/guest/upgrade`에도 전용 rate limit을 추가한다.
  - key: `guest-upgrade:user:<userID>:ip:<client-ip>`
- 두 엔드포인트는 기존 `/api/v1` 전역 rate limit과 함께 동작한다.
- lockout 정책은 이번 범위에 포함하지 않는다.
- `SessionService.Login`에는 구조화 audit log를 추가한다.
  - `event=login_attempt`
  - `username_sha256`
  - `outcome=succeeded|invalid_credentials|session_save_failed`
- `AccountService.UpgradeGuestAccount`에도 구조화 audit log를 추가한다.
  - `event=guest_upgrade_attempt`
  - `user_id`
  - `outcome=succeeded|invalid_input|invalid_token|failed`
- rate-limited outcome은 HTTP handler가 남기고, service/use case와 중복되지 않게 유지한다.
- raw username, email, password, token은 로그에 남기지 않는다.

후속 작업

- login / guest-upgrade 전용 rate limit 설정/HTTP guard 추가
- SessionService logger 주입과 login audit test 추가
- AccountService guest upgrade audit test 추가

## 2026-03-26 - signup과 guest upgrade는 verification mail 실패 시 compensating rollback으로 caller-visible atomicity를 유지한다

상태

- decided

배경

- signup과 guest upgrade는 verification mail을 durable state 커밋 이후에 보내면, 메일 실패 시 호출자에게는 실패처럼 보이지만 실제 계정 상태는 이미 바뀌는 불일치가 생긴다.
- 이전 변경은 pre-commit mail 발송 문제를 막았지만, 그 대신 signup과 guest upgrade의 성공 경계를 깨뜨렸다.

관찰

- `UserService.SignUp`은 user row 저장 후 verification token을 만들고, 메일은 after-commit 훅에서 보낸다.
- `AccountService.UpgradeGuestAccount`는 guest 승격과 token 저장을 먼저 commit한 뒤, verification mail을 after-commit 훅에서 보낸다.
- `UserRepository.Delete`는 현재 구현에서 hard delete이며, `UserRepository.Update`와 `restoreUserState()`로 guest 승격을 되돌릴 수 있다.

결론

- signup mail 실패는 user row와 verification token을 compensating transaction으로 되돌린다.
- guest upgrade mail 실패는 candidate session을 제거한 뒤, 원래 guest row를 restore하고 verification token을 무효화한다.
- caller-visible semantics는 "실패 시 durable state가 원래대로 유지된다"로 맞춘다.

후속 작업

- signup mail failure rollback 테스트 추가
- guest upgrade mail failure restore 테스트 추가
- review DB의 관련 finding 상태를 closed로 갱신

관련 문서/코드

- `internal/application/service/user/service.go`
- `internal/application/service/account/service.go`
- `internal/infrastructure/persistence/sqlite/unit_of_work.go`
- `internal/infrastructure/persistence/inmemory/unitOfWork.go`

## 2026-03-26 - signup rollback은 durable delete와 token cleanup을 분리하고, guest-upgrade rollback은 restore와 token cleanup을 분리한다

상태

- decided

배경

- signup과 guest-upgrade 보상 로직이 token cleanup 실패에 묶이면, 메인 state 복원까지 실패로 되돌아가는 문제가 남는다.
- token cleanup은 메인 state restore보다 낮은 우선순위의 정리 작업이므로, 실패해도 계정 상태 복원은 유지되어야 한다.

관찰

- signup rollback은 created user 삭제와 verification token cleanup을 같은 transaction으로 처리하고 있었다.
- guest upgrade rollback은 restore와 token cleanup을 같은 error handler 안에서 처리하고 있었다.

결론

- signup rollback은 `delete user`를 먼저 durable하게 커밋하고, token cleanup은 별도 best-effort 작업으로 분리한다.
- guest upgrade rollback은 guest row restore를 우선 보장하고, token cleanup 실패는 restore 결과를 되돌리지 않는다.
- 회귀 테스트는 mail 실패와 cleanup 실패를 별도로 주입해 state restore가 유지되는지 검증한다.

후속 작업

- signup cleanup failure test 추가
- guest-upgrade cleanup failure test 추가
- review DB의 관련 finding 상태를 다시 검증

관련 문서/코드

- `internal/application/service/user/service.go`
- `internal/application/service/account/service.go`
- `internal/application/service/userService_test.go`
- `internal/application/service/accountService_test.go`
- API/CONFIG/ARCHITECTURE/ROADMAP/Swagger 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/CONFIG.md`
- `docs/ARCHITECTURE.md`
- `internal/application/service/session/service.go`
- `internal/application/service/account/service.go`

## 2026-03-26 - search ranked cursor는 relevance cursor와 분리하고, password reset 발급은 메일 실패 시 live token이 남지 않게 처리한다

상태

- decided

배경

- ranked search 페이지는 feed-style 정렬/커서를 반환하는 반면, relevance search는 별도의 search cursor를 사용한다.
- password reset 요청은 메일 발송 실패 시에도 reset token이 storage에 남지 않아야 한다.

관찰

- `GET /api/v1/posts/search`는 `sort=relevance`와 `sort=hot|latest|top`의 cursor 형식이 다르다.
- password reset token은 저장 후 메일 발송되는 흐름이라, 발송 실패 시 토큰 정리 경계가 중요하다.

결론

- search cursor 파서는 sort family별로 분기한다.
  - `relevance`는 기존 search cursor 형식을 사용한다.
  - `hot|latest|top`은 feed cursor 형식을 사용한다.
- password reset 발급은 메일 발송 실패 시 live token이 남지 않도록, token을 먼저 pending 상태로 저장하고 발송 성공 후에만 활성화한다.
  - token 저장/무효화와 mail activation은 하나의 후속 처리 경계 안에서 처리한다.

후속 작업

- `internal/application/service/post/query_handler.go`의 ranked search cursor 분기 정리
- `internal/application/service/account/service.go`의 password reset 발급/발송 경계 회귀 테스트 추가

관련 문서/코드

- `docs/API.md`
- `internal/application/service/post/query_handler.go`
- `internal/application/service/account/service.go`

## 2026-03-27 - signup/email verification/password reset mail delivery는 outbox relay로 비동기화한다

상태

- decided

배경

- signup, email verification request, password reset request가 메일 발송 실패를 사용자-facing 요청 실패로 돌려주면, 외부 SMTP 장애가 핵심 write path를 불필요하게 흔든다.
- 요청 성공 시점에 토큰이 바로 usable 상태가 되면, outbox retry 또는 resend 경쟁에서 오래된 token이 다시 살아나는 경계가 생긴다.
- 이미 outbox relay 인프라가 있으므로, 메일 발송은 same-tx outbox 적재 + relay 소비로 넘기는 편이 기존 구조와 맞다.

관찰

- verification/reset token은 저장소에 해시로 저장되고, 신규 요청이 오면 이전 token row를 삭제해 최신 발급만 유효하게 만드는 것이 안전하다.
- relay는 at-least-once 전달이므로, 같은 token에 대한 메일이 중복 발송될 수 있다.

결론

- signup, email verification request, password reset request는 `pending token + outbox append`를 같은 transaction 안에서 처리한다.
- relay handler는 메일 발송 후 `token_hash`로 정확한 token만 활성화한다.
- token 저장소의 `InvalidateByUser`는 기존 row를 consume 표시하는 대신 삭제로 동작한다.
- 같은 사용자의 resend는 새 token을 발급하고 이전 token을 무효화하는 방식으로만 처리한다.

후속 작업

- `internal/application/event/*`
- `internal/application/service/user/service.go`
- `internal/application/service/account/service.go`
- `internal/infrastructure/persistence/{sqlite,inmemory}/*token*`
- `cmd/main.go`

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `docs/API.md`
- `internal/application/event/mail_delivery_handler.go`
- `internal/infrastructure/event/outbox/relay.go`

## 2026-03-26 - ranked cache invalidation과 ctx-aware 외부 I/O는 boundary별로 명시적으로 처리한다

상태

- decided

배경

- ranked board/tag/search 결과는 write 이벤트 이후에도 stale cache가 남으면 공개 목록의 정렬/가시성이 어긋날 수 있다.
- SMTP와 local filesystem adapter는 request/job `ctx`를 받지만, 현재는 cancel/shutdown 신호를 충분히 반영하지 않는다.

관찰

- comment/reaction 이벤트는 post feed/detail cache만 무효화하고, ranked board/tag/search cache는 남겨 둔다.
- SMTP sender는 blocking network I/O에 `ctx`를 반영하지 않고, local filesystem storage는 file I/O 전반에 `ctx`를 반영하지 않는다.

결론

- ranked cache는 normal cache와 별개 prefix로 관리하고, ranking에 영향을 주는 write 이벤트마다 해당 prefix를 지운다.
- search ranking cache는 comment/reaction 변경에도 무효화한다.
- SMTP/localfs adapter는 request/job `ctx`를 확인하고, cancel된 작업은 가능한 빨리 중단한다.

후속 작업

- `internal/application/event/cache_invalidation_handler.go`의 ranked/search prefix 무효화 추가
- `internal/infrastructure/mail/smtp/sender.go`와 `internal/infrastructure/storage/localfs/fileStorage.go`의 ctx-aware 처리 추가

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `internal/application/event/cache_invalidation_handler.go`
- `internal/infrastructure/mail/smtp/sender.go`
- `internal/infrastructure/storage/localfs/fileStorage.go`

## 2026-03-25 - password reset confirm은 부분 커밋보다 세션 무효화 우선 경계를 택한다

상태

- decided

배경

- `password reset confirm`은 비밀번호 변경과 token 소모가 먼저 반영된 뒤 세션 전체 무효화가 실패하면 부분 커밋 위험이 있었다.

관찰

- 현재 `SessionRepository`는 사용자 단위 전체 삭제는 지원하지만, 실패 후 기존 세션 집합을 복원하는 API는 없다.

결론

- `password reset confirm`은 사용자 락 아래에서 먼저 token/user 유효성을 재검증하고, 세션 전체 무효화가 성공한 뒤에만 비밀번호 변경과 reset token 소모를 commit한다.
- 즉, 세션 무효화 실패 시 비밀번호와 reset token 상태는 건드리지 않는다.
- 이 경로는 완전한 cross-store atomic transaction이 아니라 "credential state는 세션 정리가 성공했을 때만 바뀐다"는 안전 경계를 우선한다.

후속 작업

- account service 순서 재구성
- 실패 주입 테스트 추가
- ARCHITECTURE 문서 흐름 설명 갱신

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `internal/application/service/account/service.go`

## 2026-03-25 - guest issue는 active 전환 후 session 저장 순서를 따른다

상태

- decided

배경

- guest token 발급은 session 저장 후 `active` 전환이 실패하면 즉시 검증에 실패하는 unusable session을 남길 수 있었다.

관찰

- guest lifecycle은 `pending/active/expired` 상태 전이로 보상 흐름을 이미 표현하고 있다.
- `pending` 또는 `expired` guest는 인증 경로에서 유효 사용자로 인정되지 않는다.

결론

- guest token 발급은 `pending guest 생성 -> token 준비 -> active 전환 -> session 저장` 순서로 바꾼다.
- `active` 전환 실패 시 session은 저장하지 않는다.
- session 저장 실패 시 guest는 기존처럼 `expired`로 보상 전환한다.

후속 작업

- session service 순서 재구성
- 실패 주입 테스트 추가
- ARCHITECTURE 문서 흐름 설명 갱신

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `internal/application/service/session/service.go`

## 2026-03-20 - application service 비대화를 내부 협력 객체로 분해한다

상태

- decided

## 2026-03-21 - notification은 독립 도메인으로 도입하고 inbox는 사용자 조회 경험으로 둔다

상태

- decided

배경

- 현재 outbox relay 기반 이벤트 경계가 정리돼 있어, post/comment 활동을 비동기 후속 처리로 연결할 수 있는 기반은 마련돼 있다.
- 다음 사용자 경험으로는 "내 post에 comment", "내 comment에 reply", "본문에서 멘션됨" 같은 재방문 유도 흐름이 가장 직접적이다.
- 하지만 현재 단계의 outbox 구현은 in-memory relay라 외부 푸시/메일 delivery까지 바로 얹기보다 내부 확인용 inbox를 먼저 닫는 편이 운영상 안전하다.

관찰

- 기존 도메인/서비스 구조는 `board`, `post`, `comment`, `report`처럼 명시적 도메인 경계를 중심으로 정리돼 있고, HTTP는 그 결과를 사용자 경험 용어로 노출한다.
- `Inbox`는 사용자가 알림을 확인하는 read-side 개념에 가깝고, 알림 생성/읽음 처리/채널 확장 같은 lifecycle 전체를 담기에는 이름이 좁다.
- 현재 `comment.changed`, `post.changed` 이벤트에는 recipient, actor, snapshot 같은 알림 생성에 필요한 정보가 부족하다.
- 로드맵에는 Mention 이벤트와 Notification 연결이 예정돼 있으나, 실제 mention 파싱/이벤트/저장소는 아직 없다.

결론

- 새 도메인 이름은 `Notification`으로 둔다.
- `Inbox`는 별도 도메인이 아니라 사용자 자신의 notification 목록/미읽음 수를 보여주는 조회 경험 명칭으로만 사용한다.
- v1 범위는 다음 세 가지다.
  - 내 post에 comment가 달릴 때 `post_commented`
  - 내 comment에 reply가 달릴 때 `comment_replied`
  - post/comment 생성 요청에서 명시된 mention 대상이 있을 때 `mentioned`
- notification 생성은 쓰기 경로가 직접 저장하지 않고 `notification.triggered` outbox 이벤트를 relay가 소비해 비동기 적재한다.
- 외부 푸시/메일/webhook delivery는 v1 범위에서 제외하고, 내부 inbox 적재와 조회 API만 제공한다.
- 공개 API 용어는 `/users/me/notifications`로 고정하고, 응답은 UUID 기반 목록/미읽음 수/개별 읽음 처리만 연다.
- mention은 생성 시에만 처리하고 update 경로에서는 발행하지 않는다.
- self notification은 생성하지 않되, guest recipient는 허용한다.
- notification row에는 참조 ID만 두지 않고 actor 이름, post title, comment preview 최소 snapshot을 함께 저장해 원본 변경/삭제 이후에도 inbox 표시 안정성을 유지한다.

## 2026-03-21 - notification dedup은 이벤트 단위로 고정하고 mention 입력은 FE 명시 목록으로 받는다

상태

- decided

배경

- notification v1 구현은 outbox relay 재시도 시 동일 이벤트가 중복 적재되지 않아야 한다.
- 초안의 본문 파싱 기반 mention은 `@he`, `@the`처럼 일반명사형 username을 사용자가 텍스트로만 입력해도 의도치 않게 알림이 갈 수 있다.
- mention은 UX적으로 "문자열 파싱"보다 "사용자가 명시적으로 선택한 대상"이라는 의미가 더 강하다.

관찰

- relay 재시도는 동일 outbox payload를 반복 소비할 수 있으므로, notification 저장소는 재시도 중복을 막는 안정된 키가 필요하다.
- 본문 파싱은 FE mention UI 없이도 동작하지만, 오탐을 근본적으로 막기 어렵다.
- 현재 notification snapshot은 inbox 표시 안정성을 위해 최소 스냅샷을 저장한다.

결론

- notification dedup 기준은 "이벤트 단위 중복 방지"로 고정한다.
- `notification.triggered` 이벤트 payload에는 고유 `event_id`를 포함한다.
- notification 저장소는 `event_id`를 `dedup_key`로 저장하고, 동일 `event_id` 재처리는 no-op으로 취급한다.
- 별개의 사용자 액션은 내용이 동일해도 새 `event_id`를 가지므로 별도 notification으로 저장한다.
- mention 알림은 더 이상 본문 raw text를 파싱해 생성하지 않는다.
- post/comment create 요청은 FE가 명시적으로 구성한 `mentioned_usernames` 배열을 선택적으로 받는다.
- backend는 `mentioned_usernames`에 포함된 username만 대상으로 notification을 생성한다.
- 입력 목록은 trim 후 dedup하며, 존재하지 않는 username은 무시한다.
- self mention은 notification을 만들지 않는다.
- notification snapshot 길이는 v1에서 다음으로 고정한다.
  - `post_title`: 50자
  - `comment_preview`: 50자
  - 공백 trim 후 잘라 저장

후속 작업

- `notification.triggered` 이벤트에 `event_id` 추가
- notification 저장소 dedup을 `event_id` 기준으로 정렬
- post/comment create request에 `mentioned_usernames` 추가
- mention 생성 로직을 FE 명시 목록 기반으로 전환
- API/Swagger 문서에 mention 입력 규칙과 snapshot 길이 반영

관련 문서/코드

- `docs/API.md`
- `internal/application/event/types.go`
- `internal/application/event/notification_handler.go`
- `internal/application/service/common/notification_events.go`
- `internal/delivery/http_requests.go`

후속 작업

- notification entity/model/port/repository/use case 추가
- notification 전용 outbox event + serializer/relay handler 추가
- post/comment 생성 경로에서 notification trigger event 발행
- mention parser/helper와 dedup 규칙 추가
- `/users/me/notifications` 목록/미읽음 수/개별 읽음 API 추가
- 테스트, Swagger, API/ARCHITECTURE/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/ARCHITECTURE.md`

## 2026-03-23 - email verification v1은 토큰 기반 확인 + 선택적 verified-only 쓰기 제한 + 기본 SMTP 어댑터로 도입한다

상태

- decided

배경

- password reset v1 이후에도 계정 소유 증명 경로가 없어, email을 수집하는 일반 signup/guest upgrade가 여전히 미검증 상태로 남아 있다.
- 현재 쓰기 권한 정책은 suspension/guest lifecycle만 반영하고 있어, verified email을 요구하는 서비스 정책을 표현하지 못한다.
- 메일 발송 경로도 noop/test double 수준이라 실제 운영 메일 전달을 닫지 못했다.

관찰

- `signup`은 `UserService`, guest upgrade와 account lifecycle은 `AccountUseCase`에서 처리된다.
- 쓰기 계열 도메인은 post/comment/attachment/reaction/report service에서 공통 권한 경계로 제어할 수 있다.
- in-memory `UnitOfWork`는 verification token 저장소도 트랜잭션 스냅샷 경계에 포함할 수 있다.

결론

- email verification API는 두 단계로 제공한다.
  - `POST /api/v1/auth/email-verification/request`
  - `POST /api/v1/auth/email-verification/confirm`
- `signup`과 `guest upgrade` 직후 verification token을 자동 발급하고 메일을 발송한다.
- 로그인 사용자는 request API로 verification 메일을 재발송할 수 있다.
- `User`는 `EmailVerifiedAt`을 가지며, verification confirm 성공 시 이를 기록한다.
- verification token은 해시 저장 + 1회용 + 기본 TTL 30분 정책을 사용한다.
- 새 verification request가 들어오면 같은 사용자의 기존 미사용 token은 무효화한다.
- 미인증 사용자는 login/읽기/password reset/email verification 경로를 유지한다.
- guest는 기존 정책대로 `post/comment` 쓰기를 유지하고, `reaction/attachment/report`는 금지한다.
- 일반 미인증 사용자는 `post/comment/reaction`은 허용하고, verified email이 필요한 `attachment/report`만 `email verification required`로 차단한다.
- SMTP는 기본 설정 기반 단일 어댑터로 도입하고, 설정이 비활성화된 환경에서는 noop sender를 fallback으로 사용한다.
- reset/verification 메일은 같은 SMTP sender를 재사용한다.

후속 작업

- `User` verification 상태와 token 저장소 추가
- signup/guest upgrade/account request-confirm flow 반영
- SMTP 설정/어댑터와 테스트 추가
- HTTP/Swagger/API/Architecture/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/ARCHITECTURE.md`
- `internal/application/service/user/service.go`
- `internal/application/service/account/service.go`

## 2026-03-25 - email verification v2는 frontend link 메일 + request rate limit + cleanup/audit를 추가한다

상태

- decided

배경

- email verification v1은 request/confirm 기본 경로는 닫았지만, 메일 UX는 token-only이고 abuse 대응과 token lifecycle 운영 정리가 부족하다.
- password reset v2에서 메일 링크, 전용 rate limit, cleanup job, audit log 패턴을 이미 도입했으므로 verification도 같은 수준으로 맞추는 편이 일관적이다.

관찰

- 현재 verification request API는 인증 사용자 기준 단순 재발송이며, 이미 verified/ineligible 상태도 내부 no-op 처리한다.
- SMTP sender는 password reset에는 frontend 링크를 조합하지만 verification 메일은 아직 raw token만 보낸다.
- verification token 저장소에는 cleanup 연산과 background job 연결이 없다.

결론

- verification 메일은 `delivery.mail.emailVerification.baseURL` 기반 frontend 링크 + fallback token 본문으로 보낸다.
- `POST /api/v1/auth/email-verification/request`에는 `userID` 기준 전용 rate limit을 추가한다.
- 이미 verified/ineligible 상태의 request API 공개 응답은 계속 `204 no-op`로 유지한다.
- verification token 저장소에 cleanup 연산을 추가하고, background job으로 expired/consumed token을 정리한다.
- structured audit log는 account service의 request/confirm 경로에만 추가한다.
  - request: `event=email_verification_request`, `user_id`, `outcome`
  - confirm: `event=email_verification_confirm`, `user_id` 가능 시 포함, `outcome`
- `signup`/`guest upgrade` 자동 발송은 새 메일 템플릿과 token lifecycle 개선은 공유하지만 별도 audit 이벤트 대상에는 포함하지 않는다.

후속 작업

- SMTP sender / config validation 확장
- verification request 전용 rate limit 설정/HTTP guard 추가
- verification token cleanup use case/job/repository 확장
- account service audit log 추가
- HTTP/Swagger/API/CONFIG/ARCHITECTURE/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/CONFIG.md`
- `docs/ARCHITECTURE.md`
- `internal/application/service/account/service.go`
- `internal/infrastructure/mail/smtp/sender.go`

## 2026-03-25 - notification backend contract v2는 UI-friendly 응답 + read-all API로 정리한다

상태

- decided

배경

- 현재 notification inbox API는 목록 조회, unread count, 개별 read 처리만 제공한다.
- 목록 응답은 snapshot 필드만 있어 프론트가 표현 문구와 이동 타깃을 매번 규칙 기반으로 재조합해야 한다.
- 현재 레포에는 UI가 없으므로 backend-built route string보다 typed contract를 제공하는 편이 더 안전하다.

관찰

- 현재 목록 응답은 `uuid`, `type`, `actor_uuid`, `post_uuid`, `comment_uuid`, `actor_name`, `post_title`, `comment_preview`, `read_at`, `created_at`까지만 제공한다.
- unread count API와 개별 read API는 이미 존재한다.
- notification 적재는 `notification.triggered` outbox event를 통해 비동기 처리되며, type 세트는 `post_commented`, `comment_replied`, `mentioned`로 고정되어 있다.

결론

- `GET /api/v1/users/me/notifications` 응답에 아래 필드를 추가한다.
  - `is_read`
  - `target_kind`
  - `message_key`
  - `message_args`
- `target_kind`는 `post|comment`로 고정하고, `comment_uuid != nil`이면 `comment`, 아니면 `post`로 계산한다.
- `message_key`는 type별 고정값을 사용한다.
  - `post_commented -> notification.post_commented`
  - `comment_replied -> notification.comment_replied`
  - `mentioned -> notification.mentioned`
- `message_args`는 현재 snapshot 기반 object로 고정한다.
  - `actor_name`
  - `post_title`
  - `comment_preview`
- `target_path`는 추가하지 않는다. 프론트가 `target_kind + post_uuid/comment_uuid`로 라우팅을 조합한다.
- bulk read는 `PATCH /api/v1/users/me/notifications/read-all` 한 가지로 제공한다.
  - UUID 배열 기반 batch read는 이번 범위에 포함하지 않는다.
- notification 생성 이벤트와 type 세트는 유지한다.

후속 작업

- notification model/response contract 확장
- notification service query assembly 확장
- recipient 기준 bulk read 저장소/유스케이스 추가
- HTTP route / Swagger / API / ARCHITECTURE / ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/API.md`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`
- `internal/application/service/notification/service.go`
- `internal/delivery/http.go`

## 2026-03-23 - password reset v1은 email 식별자 + mail sender 포트 + 전체 세션 무효화로 도입한다

상태

- decided

배경

- 현재 일반 `signup` 계정은 `username/password`만 받아 생성되며, guest upgrade만 email을 수집한다.
- 이 상태에서는 일반 사용자 계정에 대해 email 기반 비밀번호 재설정을 제공할 수 없다.
- 계정 recovery 기능은 실제 사용자 lifecycle 완성에 직접 연결되므로 우선 구현 가치가 높다.
- 동시에 계정 존재 여부 노출과 reset token 직접 반환은 보안적으로 불필요한 공격 표면을 넓힌다.

관찰

- 현재 `User` 엔티티는 `Email` 필드를 이미 가지지만, 일반 `signup` 경로에서는 비어 있는 값으로 생성된다.
- `SessionRepository`는 사용자 단위 세션 전체 무효화(`DeleteByUser`)를 지원하므로, 비밀번호 변경 후 기존 세션을 모두 폐기할 수 있다.
- 저장소 구조는 `UnitOfWork` 기반 snapshot/rollback 모델을 사용하므로, reset token 저장소도 같은 트랜잭션 경계에 포함하는 편이 일관적이다.
- 현재 저장소에는 SMTP/mailer/reset token 관련 포트와 어댑터가 없다.

결론

- password reset v1의 대상 식별자는 `email`로 고정한다.
- 일반 `signup` 요청도 `username, email, password`를 받도록 확장한다.
- password reset API는 두 단계로 제공한다.
  - `POST /api/v1/auth/password-reset/request`
  - `POST /api/v1/auth/password-reset/confirm`
- request API는 email 형식 오류만 `400`으로 처리하고, 존재하지 않는 email/guest/deleted user는 모두 동일한 성공 응답으로 숨긴다.
- reset token은 HTTP 응답에 직접 반환하지 않고 `mail sender` 포트로만 전달한다.
- reset token 저장은 평문이 아니라 해시 저장을 기본으로 하며, token은 1회용이고 기본 TTL은 30분으로 둔다.
- 동일 사용자에 대한 새 reset request가 들어오면 이전 미사용 token은 무효화하고 최신 token만 유효하게 유지한다.
- confirm 성공 시 비밀번호 변경, token 소모, 사용자의 모든 활성 세션 무효화를 하나의 성공 경계로 처리한다.
- guest 계정과 soft-deleted 계정은 reset 대상에서 제외한다.
- 실제 SMTP 연동은 이번 범위에 포함하지 않고, port + in-memory/test double로 먼저 닫는다.

후속 작업

- `User` signup 경로에 email 수집/중복 검증 반영
- password reset token repository / token issuer / mail sender 포트 추가
- in-memory reset token 저장소와 account orchestration 구현
- HTTP/Swagger/API/Architecture/ROADMAP 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/ARCHITECTURE.md`
- `internal/application/service/account/service.go`
- `internal/application/service/user/service.go`

## 2026-03-22 - suspension 운영 API는 delivery admin middleware에서 먼저 차단한다

상태

- decided

배경

- `GET/PUT/DELETE /api/v1/users/{userUUID}/suspension`는 문서상 admin 전용 운영 API다.
- 하지만 현재 라우팅은 인증 미들웨어만 거치고, 실제 admin 판정은 `UserService` 내부에서 수행한다.
- 이 순서에서는 non-admin 요청도 handler 안으로 들어와 UUID/body 검증을 먼저 거치므로, `400`과 `403` 차이를 통해 admin surface의 입력 검증 결과를 관찰할 수 있다.

관찰

- 아키텍처 문서는 HTTP admin middleware를 포함한 권한 확인 경계를 명시한다.
- suspension use case는 service 내부 `AdminOnly` 체크를 이미 가지고 있어 최종 권한은 막고 있지만, 이는 defense-in-depth여야지 delivery의 1차 경계 대체가 되어서는 안 된다.
- 기존 malformed UUID 검증은 admin caller에게는 계속 유지되어야 한다.

결론

- suspension GET/PUT/DELETE 라우트는 `authGinMiddleware`와 함께 `adminGinMiddleware`를 적용한다.
- non-admin 호출은 path/body 검증 전에 `403 Forbidden`으로 종료한다.
- service 레이어의 `AdminOnly` 검사는 유지해 우회 호출 시에도 동일 invariant를 보장한다.
- admin caller에 대해서만 기존 UUID/body 검증과 use case 호출 흐름을 유지한다.

후속 작업

- suspension 라우트에 admin middleware 추가
- non-admin 조기 차단 회귀 테스트 추가
- 전체 HTTP/통합 테스트로 기존 admin 성공/검증 흐름이 유지되는지 확인

관련 문서/코드

- `internal/delivery/http.go`
- `internal/delivery/http_test.go`
- `internal/application/service/user/service.go`
- `docs/ARCHITECTURE.md`

## 2026-03-22 - admin use case의 사용자 조회와 AdminOnly 검증은 공통 helper로 수렴한다

상태

- decided

배경

- suspension 라우트는 delivery에서 먼저 admin middleware로 차단하도록 정리했지만, application service 내부에도 admin 사용자 조회와 `AdminOnly` 검증이 여러 서비스에 반복돼 있다.
- 현재 `board`, `report`, `outboxadmin`, `user` 서비스는 모두 `SelectUserByID -> nil 확인 -> authorizationPolicy.AdminOnly(...)`를 거의 같은 형태로 반복한다.
- 이 구조는 방어 심도 자체는 유지하지만, 에러 문구나 nil 처리, 이후 정책 확장 시 드리프트가 생기기 쉽다.

관찰

- service 레이어의 admin 체크는 delivery를 우회하는 호출에 대한 invariant로 유지할 가치가 있다.
- 반면 동일한 조회/검증 시퀀스를 서비스마다 풀어 쓰는 것은 구조적으로 중복이다.
- 이미 `service/common`은 여러 서비스가 공유하는 helper를 수용하는 위치로 정의돼 있다.

결론

- service 레이어의 admin invariant는 유지한다.
- admin 사용자 조회와 `AdminOnly` 판정은 `service/common`의 공통 helper로 수렴한다.
- 각 서비스는 helper에 operation context만 넘겨 repository wrapping 문구를 유지하고, nil 처리와 정책 적용은 helper가 담당한다.
- delivery는 조기 차단, service는 공통 helper 기반 invariant라는 이중 경계를 유지한다.

후속 작업

- `service/common`에 admin require helper 추가
- `board`, `report`, `outboxadmin`, `user` 서비스의 중복 로직을 helper 사용으로 치환
- helper 단위 테스트와 관련 서비스 회귀 테스트로 기존 권한 동작 유지 확인

관련 문서/코드

- `internal/application/service/common`
- `internal/application/service/board/service.go`
- `internal/application/service/report/service.go`
- `internal/application/service/outboxadmin/service.go`
- `internal/application/service/user/service.go`
- `internal/application/event/types.go`
- `internal/application/service/post/command_handler.go`
- `internal/application/service/comment/command_handler.go`
- `internal/delivery/http.go`

## 2026-03-20 - post 검색 v1은 별도 search port와 in-memory reference adapter로 시작한다

상태

- decided

배경

- post 검색은 현재 in-memory 단계에서도 API 계약을 먼저 고정할 필요가 있었고, 이후 Phase 2에서 SQLite FTS5로 전환할 예정이라 저장소 구현을 바로 고정하지 않는 편이 유리했다.
- tag 조회와 달리 검색은 랭킹, phrase boost, tokenizer, cursor 정렬 규칙까지 함께 커질 가능성이 높아 `PostRepository`에 계속 질의를 누적하면 read-side 계약이 비대해질 우려가 있었다.
- 한국어/영어를 모두 다뤄야 하지만, 현재 단계에서 형태소 분석까지 넣는 것은 과한 범위였다.

관찰

- 현재 공개 목록 API는 opaque `cursor`를 사용하고, `PostService`의 read path는 `query handler`에서 visibility/pagination 조립 책임을 가진다.
- 로드맵에는 SQLite FTS5 기반 전문 검색이 예정돼 있다.
- 공개 post 목록 응답은 이미 `PostList`로 안정화돼 있어, v1 검색도 같은 응답 shape를 재사용하는 편이 변경 비용이 작다.

결론

- 검색 경계는 `PostRepository` 확장이 아니라 별도 `PostSearchRepository`로 분리한다.
- 검색 조회 경계와 인덱스 갱신 경계를 분리하기 위해 `PostSearchIndexer`를 함께 둔다.
- v1 검색 대상은 `published` post의 `title + content + tag names`로 한정한다.
- 토큰화는 한국어/영어 구분 없이 whitespace split + lowercase로 시작한다.
- 기본 매칭 규칙은 `all terms match`로 한다.
- 랭킹은 BM25 기반으로 하고, field weight는 `title > tag > content`를 적용한다.
- exact phrase는 필수 조건이 아니라 optional boost로만 취급한다.
- 공개 API는 `GET /api/v1/posts/search?q=&limit=&cursor=`로 추가하고, 응답은 기존 `PostList`를 재사용한다.
- 검색 pagination 정렬은 `score desc, post_id desc`로 고정하고, 공개 cursor는 opaque 형태를 유지하되 내부 payload는 composite cursor를 사용한다.
- v1 구현은 in-memory reference adapter로 먼저 제공하되, 조회 시 매번 원본 저장소를 스캔하지 않고 search document 인덱스를 유지한다.
- post 인덱스 갱신은 `post.changed` outbox 이벤트를 relay가 소비하는 비동기 흐름으로 처리한다.
- board visibility는 검색 저장소 구현이 아니라 application read path에서 계속 필터링한다.
- 이후 SQLite FTS5, Elasticsearch, Meilisearch adapter는 동일한 `PostSearchRepository + PostSearchIndexer` 계약을 구현한다.
- wiring에서는 `query`와 `index`가 서로 다른 구현체로 분리되지 않도록, 하나의 concrete search store 인스턴스를 두 포트로 함께 주입한다.

후속 작업

- `PostUseCase`/delivery/search port 계약 추가
- `PostSearchIndexer` 포트와 outbox event consumer 추가
- in-memory `PostSearchRepository`/`PostSearchIndexer` 구현
- service/query handler 검색 orchestration 및 visibility filtering 추가
- HTTP/서비스/저장소 테스트 추가
- API/Swagger/로드맵 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `internal/application/port/post_usecase.go`
- `internal/application/port/post_search_repository.go`
- `internal/application/service/post/query_handler.go`
- `internal/infrastructure/persistence/inmemory/`

배경

- `PostService`는 조회 조립, 태그 동기화, attachment 참조 검증, 삭제 workflow, 이벤트 append까지 함께 들고 있어 application 레이어의 책임 분리가 약해지고 있었다.
- `CommentService`, `AttachmentService`도 같은 방향으로 command/query/policy가 한 타입 안에 누적되는 경향이 보였다.
- 공개 API와 use case port는 안정화되어 있으므로, 이번 작업은 외부 계약을 바꾸지 않고 내부 구조만 재정렬하는 방식이 적절했다.

관찰

- 현재 구조는 helper 일부(`postDetailQuery`, `comment_projection`, `visibility_helper`)를 이미 추출했지만, 서비스 메서드가 여전히 read assembly/policy/workflow를 직접 조합한다.
- `application` 레이어는 이 프로젝트에서 orchestration, tx boundary, cache policy를 유지해야 하므로, 레이어 자체를 줄이기보다 서비스 내부 책임을 더 작은 협력 객체로 나누는 편이 맞다.

결론

- `PostService`는 facade로 유지하고, 실제 로직은 `postCommandHandler`, `postQueryHandler`, `postTagCoordinator`, `postAttachmentCoordinator`, `postDeletionWorkflow`로 분해한다.
- `CommentService`, `AttachmentService`는 같은 패턴으로 command/query를 우선 분리한다.
- guest write guard, board visibility concealment 같은 재사용 규칙은 서비스 전용 helper가 아니라 `application/policy`에 둔다.
- 공개 계약은 유지한다.
  - `port.PostUseCase`, `port.CommentUseCase`, `port.AttachmentUseCase`
  - HTTP path/body/response
  - UUID/int64 정책
  - outbox/cache 의미론
- 비목표는 다음과 같다.
  - CQRS 도입
  - delivery/API 변경
  - SQLite 전환
  - 이벤트 의미 변경

후속 작업

- `PostService` characterization test 보강
- `PostService` 내부 handler/coordinator/workflow 타입 도입
- `CommentService`, `AttachmentService` command/query 분리
- `docs/ARCHITECTURE.md`에 service 비대화 방지 원칙 반영

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `internal/application/service/postService.go`
- `internal/application/service/commentService.go`
- `internal/application/service/attachmentService.go`
- `internal/application/policy/board_visibility_policy.go`

## 2026-03-20 - reaction/report 분해는 진행하고 service 폴더 분리는 별도 단계로 분리한다

상태

- decided

배경

- `Post`, `Comment`, `Attachment` 분해 이후에도 `ReactionService`, `ReportService`는 command/query/workflow가 한 타입에 남아 있었다.
- 동시에 `internal/application/service` 디렉터리에는 도메인별 파일이 섞여 있어, 서비스가 늘수록 탐색 비용이 커질 가능성이 보였다.
- 하지만 Go에서는 폴더 분리가 곧 패키지 경로 분리로 이어지므로, 이번 리팩토링과 물리 디렉터리 재배치를 한 번에 섞으면 영향 범위가 커진다.

관찰

- `ReactionService`, `ReportService`는 현재 공개 생성자와 use case port가 이미 여러 진입점에서 사용된다.
- 디렉터리 재배치는 `cmd/main.go`, integration test, delivery wiring의 import 경로까지 함께 바뀌게 만든다.
- 반면 내부 handler 분해는 공개 계약 유지 상태에서도 바로 진행할 수 있다.

결론

- 이번 단계에서는 `ReactionService`, `ReportService`를 facade + 내부 handler/query 구조로 분해한다.
- 물리 디렉터리 분리는 이번 작업 범위에서 제외한다.
- 당장은 파일명 prefix(`reaction_*`, `report_*`, `post_*`, `comment_*`, `attachment_*`)로 도메인 경계를 유지한다.
- 이후 별도 작업에서 `internal/application/service/<domain>` 형태의 패키지 재배치를 검토한다.
  - 그 단계에서는 facade import 경로 변경
  - composition root/wiring 수정
  - 순환 의존 여부 점검
  - 패키지 public surface 재정의
  를 함께 처리한다.

후속 작업

- `ReactionService`, `ReportService` command/query 분해
- 파일명 prefix 규칙을 아키텍처 문서에 반영
- 별도 backlog 또는 결정 문서에서 도메인별 패키지 분리 기준 정리

관련 문서/코드

- `internal/application/service/reactionService.go`
- `internal/application/service/reportService.go`
- `cmd/main.go`
- `internal/integration/http_flow_test.go`

## 2026-03-20 - 도메인별 패키지 재배치 전 공통 service helper를 별도 패키지로 먼저 분리한다

상태

- decided

배경

- 도메인별 하위 패키지(`service/post`, `service/comment` 등)로 이동하려면 현재 `service` 루트에 섞여 있는 공통 helper를 먼저 걷어내야 한다.
- 공통 helper가 루트에 남아 있으면 하위 패키지 이동 시 순환 의존이나 광범위한 상대 import가 생길 가능성이 높다.

관찰

- 현재 `cache_errors`, `cursor_list`, `pagination`, `public_cursor`, `user_reference`, `event_publisher`, `outbox_events`, `logger`는 특정 도메인보다 여러 서비스가 함께 사용한다.
- 이 파일들은 도메인 행동이라기보다 application service 공통 도구에 가깝다.

결론

- 도메인별 물리 재배치 전에 `internal/application/service/common` 패키지를 도입한다.
- 공통 helper는 먼저 `common` 패키지로 옮기고, 기존 서비스는 이 패키지를 import 하게 바꾼다.
- 이후 도메인 서비스 하위 패키지로 내릴 때는 각 도메인이 `common`만 의존하도록 정리한다.

후속 작업

- `service/common` 패키지 도입 및 공용 helper 이동
- 기존 서비스/테스트 import 정리
- 이후 단계에서 `service/<domain>` 패키지 재배치

관련 문서/코드

- `internal/application/service/cache_errors.go`
- `internal/application/service/cursor_list.go`
- `internal/application/service/pagination.go`
- `internal/application/service/public_cursor.go`
- `internal/application/service/user_reference.go`
- `internal/application/service/event_publisher.go`
- `internal/application/service/outbox_events.go`

## 2026-03-20 - reaction/report는 실제 하위 패키지로 먼저 이동하고 루트 service는 호환 wrapper로 유지한다

상태

- decided

배경

- `service/common` 분리 이후에는 실제 하위 패키지 이동을 작은 범위로 시작할 수 있게 되었다.
- `reaction`, `report`는 다른 도메인보다 의존 폭이 상대적으로 좁고, `post/comment/attachment`보다 조합 복잡도도 낮아 패키지 이동의 첫 타깃으로 적합했다.

관찰

- 외부 wiring은 여전히 `internal/application/service` 경로의 생성자에 의존한다.
- `cmd/main.go`, delivery/integration test는 `service.NewReactionServiceWithActionDispatcher`, `service.NewReportServiceWithActionDispatcher`를 직접 사용한다.
- 따라서 패키지 이동과 동시에 외부 import 경로를 바꾸는 대신, 루트 package는 wrapper만 남기고 실제 구현을 하위 패키지로 내리는 방식이 회귀 위험을 낮춘다.

결론

- `internal/application/service/reaction`, `internal/application/service/report` 하위 패키지에 실제 구현을 둔다.
- 루트 `internal/application/service`에는 기존 생성자와 타입 경로 호환을 위한 thin wrapper만 유지한다.
- component characterization test는 하위 패키지의 exported handler constructor를 직접 사용하게 바꾼다.
- 이 패턴을 기준으로 이후 다른 도메인도 순차 이동하되, 의존 폭이 큰 `post/comment/attachment`는 별도 단계로 다룬다.

후속 작업

- `reaction/report` 하위 패키지 이동 반영
- 루트 service wrapper 유지
- architecture 문서에 partial package split 상태 반영
- 이후 `post/comment/attachment` 재배치 범위 검토

관련 문서/코드

- `internal/application/service/reactionService.go`
- `internal/application/service/reportService.go`
- `internal/application/service/reaction/service.go`
- `internal/application/service/report/service.go`

## 2026-03-20 - 남은 application service도 모두 하위 패키지로 이동하고 루트 service는 facade만 유지한다

상태

- decided

배경

- `reaction/report` 이동 이후에도 `post/comment/attachment/board/user/session/account/guestCleanup/outboxAdmin` 구현이 루트 `service`에 남아 있어 탐색 기준이 혼재되어 있었다.
- 공용 helper는 이미 `service/common`으로 분리됐기 때문에, 남은 서비스도 같은 패턴으로 하위 패키지로 내릴 준비가 되어 있었다.

관찰

- 외부 wiring과 대부분의 테스트는 여전히 `internal/application/service`의 생성자와 타입 이름에 의존한다.
- `post`, `comment`, `attachment`는 handler/coordinator/workflow/component test가 존재해, 하위 패키지 이동과 동시에 테스트 호환 경계도 정리해야 했다.

결론

- `post`, `comment`, `attachment`, `board`, `user`, `session`, `account`, `guestcleanup`, `outboxadmin` 구현을 각각 `service/<domain>` 하위 패키지로 이동한다.
- 루트 `internal/application/service`는 공개 생성자와 타입 alias만 유지한다.
- `post/comment/attachment` 하위 패키지는 exported constructor/helper를 제공하고, 테스트도 하위 패키지를 직접 import해 사용한다.
- 최종 구조 기준은 다음과 같다.
  - 다도메인 공용 helper: `service/common`
  - 도메인 구현: `service/<domain>`
  - 외부 wiring 진입점: 루트 `service`

후속 작업

- 남은 도메인 하위 패키지 이동 반영
- architecture 문서를 최종 패키지 구조 기준으로 갱신
- 루트 테스트 호환 wrapper 제거

관련 문서/코드

- `internal/application/service/post/service.go`
- `internal/application/service/comment/service.go`
- `internal/application/service/attachment/service.go`
- `internal/application/service/board/service.go`
- `internal/application/service/user/service.go`
- `internal/application/service/session/service.go`
- `internal/application/service/account/service.go`
- `internal/application/service/guestcleanup/service.go`
- `internal/application/service/outboxadmin/service.go`

## 2026-03-13 - 로컬 터미널 툴체인과 AGENTS 기본 CLI 규칙을 표준화한다

상태

- decided

배경

- 로컬 개발자 작업 환경과 AGENTS의 자동화 효율을 높이려면, 반복적으로 쓰는 탐색/검색/출력/검증 도구를 일관되게 맞출 필요가 있었다.
- 특히 파일 탐색, 텍스트 검색, 구조 검색, GitHub 작업, JSON/YAML 파이프라인 처리에서 더 빠르고 비대화형에 적합한 CLI 조합이 필요했다.

관찰

- 현재 저장소의 `AGENTS.md`는 작업 순서와 skill 사용 규칙은 정의하지만, 구체적인 CLI 기본값과 비대화형 실행 원칙은 아직 충분히 명시하지 않는다.
- macOS 개발 환경에서는 Homebrew를 통해 Rust 기반 CLI와 TUI 도구를 일관되게 배포하고 갱신할 수 있다.

결론

- 로컬 기본 툴체인은 `rg`, `fd`, `bat`, `eza`, `zoxide`, `fzf`, `atuin`, `jq`, `yq`, `httpie`, `ast-grep`, `difftastic`, `shellcheck`, `shfmt`, `ruff`, `gh`, `git-delta`, `lazygit`, `yazi`, `starship`로 표준화한다.
- AGENTS는 파일 탐색 시 `fd`, 텍스트 검색 시 `rg`, 파일 읽기 시 `bat`, 디렉터리 목록 확인 시 `eza`를 우선 사용한다.
- 구조 기반 검색/리팩토링은 `ast-grep`를 우선 검토한다.
- JSON/YAML/API 응답 처리는 `jq`, `yq`, `httpie` 조합을 우선 사용한다.
- GitHub 관련 자동화는 `gh`를 비대화형 JSON 출력 중심으로 사용한다.
- 외부 서비스와 상호작용하는 CLI는 가능한 한 `--yes`, `--quiet`, `--json`, `--format json` 같은 비대화형/기계 판독 옵션을 강제한다.

후속 작업

- `AGENTS.md`에 위 기본 CLI 규칙 반영
- 설치된 도구 버전 검증
- 개발자 셸 초기화 예시 정리

관련 문서/코드

- `AGENTS.md`
- `docs/DECISIONS.md`

## 2026-03-08 - 도메인 도입 우선순위 및 기존 엔티티 보강 방향

상태

- decided

배경

- `docs/ROADMAP.md`에는 Step 2 확장 도메인으로 `Attachment`, `Report`, `Notification`, `PointHistory`, `Tag`가 정의되어 있다.
- 현재 레포는 `user`, `board`, `post`, `comment`, `reaction`, `session`, `account` 중심으로 구성되어 있다.
- 신규 도메인을 추가하기 전에, 현재 코어 엔티티가 운영성 요구를 얼마나 수용할 수 있는지 점검할 필요가 있었다.

관찰

- `User`는 soft delete + 익명화는 지원하지만, 이메일 인증/비밀번호 재설정/정지 같은 생명주기 상태를 담을 구조는 아직 없다.
- 당시 검토 기준으로 `Post`는 `Title`, `Content`, `AuthorID`, `BoardID`, 시간 필드만 가지고 있어 `draft`, `soft delete`, `slug`, moderation 상태를 표현하기 어렵다고 봤다.
- `Comment`는 `ParentID`는 이미 있으나 실제 유스케이스와 요청 모델은 대댓글 입력을 아직 받지 않는다.
- `Comment` 역시 상태 필드와 삭제 시각이 없어 soft delete, 신고 처리, moderation 확장에 불리하다.
- `Notification`과 `PointHistory`는 이벤트 발행 구조가 없는 현재 구조에서는 이르게 도입하면 서비스 간 결합이 커질 가능성이 높다.
- `Attachment`와 `Tag`는 상대적으로 독립성이 높지만, 결국 `Post` 또는 `Comment`의 상태 모델과 결합될 가능성이 있다.

결론

- 신규 도메인을 바로 늘리기보다, 먼저 기존 코어 엔티티를 보강한다.
- 우선 보강 대상은 `User`, `Post`, `Comment`다.
- `User`는 이메일 인증, 비밀번호 재설정, 정지 정책을 수용할 수 있도록 생명주기 모델을 확장한다.
- 당시에는 `Post` 상태 모델에 `draft`, `soft delete`, `slug`, moderation 확장을 함께 검토했다.
- 현재 공개 식별자 정책은 `slug`가 아니라 `uuid`를 채택한다.
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

## 2026-03-11 - 운영 안정성 보강(요청 크기 경계/이벤트 백프레셔/UoW 비용/레이스 안정화)

상태

- decided

배경

- 기능 정확성 테스트(`go test ./...`)는 통과하지만, 운영 트래픽/데이터가 증가할 때 리소스 소모와 동시성 안정성 측면의 보강이 필요했다.
- 품질 점검 중 JSON 요청 바디 상한 부재, 이벤트 발행 고루틴 폭증 가능성, in-memory UoW 전체 스냅샷 비용, `-race` 기준 테스트 유틸 레이스가 확인됐다.

관찰

- `decodeJSON`는 strict decode를 수행하지만 바디 크기 상한을 강제하지 않는다.
- in-process event bus는 publish 호출마다 비동기 고루틴을 생성한다.
- in-memory `UnitOfWork`는 트랜잭션마다 모든 저장소 상태를 깊은 복사한다.
- `runner_test`의 `stubTickerFactory`가 동시 접근 보호 없이 슬라이스를 읽고 쓴다.

결론

- JSON 엔드포인트에 공통 요청 바디 상한을 도입한다.
- 이벤트 버스는 bounded queue + 고정 worker 기반으로 바꿔 백프레셔를 둔다.
- UoW는 write가 발생한 저장소만 lazy snapshot 하도록 변경한다.
- 레이스 감지 기준에서 신뢰 가능한 테스트가 되도록 테스트 유틸 동기화를 추가한다.

후속 작업

- delivery JSON oversize 거부 테스트 추가
- event bus queue full 드롭/로그 테스트 추가
- UoW lazy snapshot 반영 후 기존 트랜잭션 회귀 테스트 통과 확인
- `go test -race ./...` 통과 확인

관련 문서/코드

- `internal/delivery/http.go`
- `internal/delivery/http_test.go`
- `internal/infrastructure/event/inprocess/event_bus.go`
- `internal/infrastructure/event/inprocess/bus_test.go`
- `internal/infrastructure/persistence/inmemory/unitOfWork.go`
- `internal/infrastructure/job/inprocess/runner_test.go`

## 2026-03-11 - 운영 튜닝 가능성 강화(JSON 제한/이벤트 버스 설정/쓰기 병렬 계측)

상태

- decided

배경

- 안정성 보강 이후에도 운영 관점에서 제한값/처리량을 환경별로 조정할 수 있는 지점이 부족했다.
- 특히 JSON 바디 제한과 이벤트 버스 큐/워커 수가 코드 기본값에 고정되어 있어 서비스 프로파일별 튜닝이 어렵다.
- UoW의 write 직렬화는 구조적으로 남아 있어, 개선 전/후를 비교할 수 있는 기준 벤치가 필요하다.

관찰

- JSON 바디 제한은 현재 delivery 내부 상수로 적용된다.
- 이벤트 버스 queue/worker 기본값은 코드에 있으나 config에서 조절하지 않는다.
- UoW는 write 트랜잭션을 전역 mutex로 직렬화한다.

결론

- JSON 바디 상한은 config로 승격한다.
- 이벤트 버스 queue size / worker count를 config로 노출한다.
- 이벤트 드롭을 테스트 가능/관측 가능하게 stats를 제공한다.
- UoW는 즉시 동시화 구조를 바꾸기보다 병렬 write 벤치마크를 먼저 도입해 기준선을 만든다.

후속 작업

- `delivery.http.maxJSONBodyBytes` 설정 추가 및 문서 반영
- `event.inprocess.queueSize`, `event.inprocess.workerCount` 설정 추가 및 wiring
- 이벤트 버스 드롭 카운터 테스트/노출 추가
- 이벤트 큐 포화 시 block + timeout 정책 도입 및 설정화
- UoW 병렬 벤치마크 추가

관련 문서/코드

- `internal/config/config.go`
- `cmd/main.go`
- `internal/delivery/http.go`
- `internal/infrastructure/event/inprocess/event_bus.go`
- `internal/infrastructure/persistence/inmemory/*`

## 2026-03-11 - EventBus 종료 경계와 아키텍처 문서 동기화

상태

- decided

배경

- 이벤트 버스가 worker goroutine을 상시 유지하지만 종료 API가 없어 graceful shutdown 경계가 불명확했다.
- 운영 문서(`CONFIG`, `API`)에는 최신 정책이 반영됐지만, 아키텍처 문서에는 일부 런타임 정책(이벤트 큐 포화 드롭, JSON 바디 제한 책임)이 충분히 명시되지 않았다.

관찰

- EventBus는 queue 기반 비동기 처리만 제공하고 lifecycle close가 없다.
- 큐 포화 시 block + timeout 후 drop + warn 정책으로 전환한다.
- JSON 바디 제한은 delivery 경계에서 적용되고 설정으로 조절 가능하다.

결론

- EventBus에 `Close()`를 추가해 worker lifecycle을 명시적으로 종료할 수 있게 한다.
- `Close()` 이후 publish는 drop으로 처리하고, stats/warn로 관측 가능하게 유지한다.
- 이벤트 큐 포화 동작은 `block -> timeout -> drop`으로 명시한다.
- `docs/ARCHITECTURE.md`에 이벤트 포화 정책과 JSON 제한 책임/설정 키를 반영한다.
- UoW는 전역 coordinator lock 대신 repository 단위 coordinator lock으로 바꿔 lock granularity를 개선한다.

후속 작업

- EventBus close 동작/종료 후 publish 정책 테스트 추가
- composition root 종료 경로에서 EventBus close 호출
- 아키텍처 문서에 runtime 정책(드롭/제한) 반영

관련 문서/코드

- `internal/infrastructure/event/inprocess/event_bus.go`
- `internal/infrastructure/event/inprocess/bus_test.go`
- `cmd/main.go`
- `docs/ARCHITECTURE.md`

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
- 당시 공개 응답은 flat list + `parent_id` 노출을 유지하는 방향으로 정리했다.
- 현재 공개 계약은 UUID 전환에 따라 flat list + `parent_uuid` 노출을 사용한다.

후속 작업

- comment create API에 부모 연결 값 추가
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

- 당시 결정 기준으로 본문은 Markdown 이미지 문법의 attachment 참조를 직접 가진다.
- 현재 공개 계약의 참조 형식은 `attachment://{attachmentUUID}` 이다.
- upload API는 본문에 바로 넣을 수 있는 `embed_markdown`을 응답한다.
- `PostDetail`은 attachment 목록을 함께 내려 클라이언트가 본문 참조를 해석할 수 있게 한다.
- attachment 응답에는 실제 파일 조회용 `file_url`을 포함한다.
- 1차 파일 조회는 published post 기준 public read 경로로 연다.
- draft 작성 중 미리보기는 owner/admin 전용 authenticated `preview_url`로 제공한다.
- 파일 캐시는 앱 메모리 캐시보다 HTTP 캐시 헤더를 우선 적용한다.
- `file_url`은 `Cache-Control: public` + `ETag`를 사용하고, `preview_url`은 `private, no-store`로 둔다.
- attachment 업로드는 우선 이미지 화이트리스트(`png`, `jpeg/jpg`, `gif`, `webp`)와 설정 가능한 최대 크기 제한을 둔다.
- 업로드 MIME은 요청 헤더만 믿지 않고 본문 sniffing 결과와 일치해야 한다.
- storage key는 같은 파일명 충돌을 피하기 위해 내부 post 식별자 기반 경로(`postID/랜덤-sanitized-name`)를 사용한다.
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

## 2026-03-11 - ROADMAP 상태 표기를 outbox 전환 기준으로 동기화

상태

- decided

배경

- `in-process publish -> outbox append + relay` 전환이 완료되었지만, `docs/ROADMAP.md`의 Step 4/5 상태 표기가 과거 계획 기준으로 남아 있었다.
- 문서상 현재 상태와 다음 우선순위가 어긋나면 후속 작업(특히 SQLite outbox adapter 전환)의 착수 지점이 불명확해진다.

관찰

- ROADMAP Step 4는 여전히 "in-process event bus 도입" 수준으로 표현되어 있다.
- 실제 코드는 tx 내부 outbox append, relay retry/backoff/dead 정책까지 반영되어 있다.

결론

- ROADMAP은 Step 4를 "outbox 경로 전환 완료(인-memory relay)" 기준으로 갱신한다.
- Step 5에는 다음 내구화 범위(SQLite outbox table + relay/CDC, MQ bridge 선택)를 명시한다.
- 남은 항목은 "done/remaining" 기준으로 분리해 현재 단계와 후속 단계를 명확히 구분한다.

후속 작업

- SQLite outbox adapter 도입 시 ROADMAP Step 5 진행 상태 업데이트
- dead 이벤트 재처리 운영 경로 확정 후 문서 반영

관련 문서/코드

- `docs/ROADMAP.md`
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

## 2026-03-10 - PostDetail read assembly는 service에서 query component로 분리한다

상태

- decided

배경

- `PostService.GetPostDetail`은 cache orchestration 외에도 post, tags, attachments, comments, reactions, user UUID expansion까지 직접 조립하고 있었다.
- 현재 in-memory 구현에서는 동작하지만, 읽기 경로 최적화와 비즈니스 규칙이 한 메서드에 같이 커지는 구조였다.

관찰

- `GetPostDetail`의 핵심 use case 책임은 cache boundary와 공개 contract 유지이지, 세부 read assembly 자체는 아니다.
- read model 조립을 별도 component로 분리하면 서비스는 orchestration에 집중하고, 조회 최적화나 projection 규칙은 query 쪽에서 독립적으로 다룰 수 있다.

결론

- `PostService.GetPostDetail`은 cache/use case 경계를 유지하고, 실제 read assembly는 별도 `postDetailQuery` component가 담당한다.
- post/comment/reaction projection 규칙은 query와 service가 공통 helper를 공유해 중복 없이 유지한다.
- 외부 use case contract와 응답 스키마는 변경하지 않는다.

후속 작업

- post detail read assembly helper를 query component로 이동
- query component 직접 테스트 추가
- 아키텍처 문서에 read-side query helper 사용 원칙 반영

관련 문서/코드

- `internal/application/service/postService.go`
- `internal/application/service/post_detail_query.go`
- `docs/ARCHITECTURE.md`

## 2026-03-10 - Swagger 검증은 명시적 품질 게이트에만 포함한다

상태

- decided

배경

- Swagger 산출물(`docs/swagger`)은 public API 문서 surface지만, 런타임 필수 자산은 아니다.
- 매 커밋/실행/테스트마다 swagger 재생성을 강제하면 개발 루프를 불필요하게 느리게 만들 수 있다.
- 반대로 아무 검증도 없으면 annotation과 generated docs가 쉽게 드리프트한다.

관찰

- 이 저장소는 이미 `테스트 -> 문서 정합성 반영` 순서를 중시하므로, Swagger도 최종 검증 단계에 포함하는 것이 자연스럽다.
- 특정 CI 또는 Git hosting 기능에 의존하지 않는 로컬 검증 진입점이 있으면 환경 독립적으로 같은 규칙을 적용할 수 있다.

결론

- Swagger 검증은 일상 개발 루프 전체에 강제하지 않고 `make verify` 같은 명시적 품질 게이트에만 포함한다.
- 로컬에서 선택적으로 사용할 수 있는 git hook 스크립트는 제공하되 자동 설치는 하지 않는다.
- 검증 스크립트는 `make swagger` 실행 후 `docs/swagger` diff가 남는지 검사하는 방식으로 유지한다.

후속 작업

- `scripts/verify-swagger.sh` 추가
- `make verify`에 test/vet/swagger verification 연결
- 선택적 `githooks/pre-commit` 및 설치 스크립트 추가
- README와 문서에 사용법 반영

관련 문서/코드

- `Makefile`
- `README.md`
- `docs/API.md`

## 2026-03-10 - Swagger source annotation과 object storage upload path를 실제 계약에 맞춘다

상태

- decided

배경

- Swagger 산출물 검증을 추가한 뒤에는 generated docs drift뿐 아니라 source annotation 자체의 정확성도 중요해졌다.
- attachment upload는 서비스 계층에서 이미 size-bound buffering을 한 번 수행하므로, backend adapter가 같은 payload를 다시 full copy하면 메모리 피크가 불필요하게 커진다.

관찰

- 일부 handler의 Swagger `@Success` annotation이 실제 응답 DTO와 어긋나면 생성 산출물도 일관되게 틀린 계약을 노출한다.
- object storage adapter는 `PutObject` 직전에 `io.ReadAll`로 전체 파일을 다시 읽고 있다.

결론

- Swagger source annotation은 handler가 실제로 반환하는 response DTO와 정확히 일치시킨다.
- attachment upload의 object backend 경로는 가능한 경우 원본 reader와 known size를 그대로 `PutObject`에 전달해 추가 full copy를 피한다.
- 이 두 계약은 테스트로 고정한다.

후속 작업

- Swagger response schema 회귀 테스트 추가
- object storage adapter 전달 방식 회귀 테스트 추가
- handler annotation 수정 및 swagger regenerate
- object storage adapter 최적화

관련 문서/코드

- `internal/delivery/http.go`
- `docs/swagger/`
- `internal/infrastructure/storage/object/fileStorage.go`

## 2026-03-11 - Swagger 검증은 파일 상태가 아니라 diff 본문을 비교한다

상태

- decided

배경

- `scripts/verify-swagger.sh`는 `docs/swagger`의 상태 문자열(`git status --porcelain`) 전후 비교로 정합성을 판단했다.
- 이 방식은 `docs/swagger`가 이미 수정된 dirty worktree에서 추가 변경이 생겨도 상태 문자열이 동일하면 놓칠 수 있다.

관찰

- `make swagger` 실행 전후 모두 `M docs/swagger/...`처럼 같은 상태 줄이 유지되면 실제 내용 변경 여부를 판별할 수 없다.
- 목표는 "현재 작업 트리가 깨끗한가"가 아니라 "생성 실행으로 추가 변화가 생겼는가"를 감지하는 것이다.

결론

- 검증 기준을 상태 문자열 비교에서 `git diff -- docs/swagger` 본문 전후 비교로 변경한다.
- dirty worktree에서도 생성 실행으로 diff 본문이 바뀌면 검증 실패로 처리한다.

후속 작업

- `scripts/verify-swagger.sh` 비교 로직 수정
- `make verify`로 회귀 확인

관련 문서/코드

- `scripts/verify-swagger.sh`
- `Makefile`

## 2026-03-11 - EDA 1차 최소 세트로 캐시 무효화를 이벤트 소비 지점으로 수렴한다

상태

- decided

배경

- 현재 write 유스케이스는 서비스마다 `best effort` 캐시 무효화 호출이 분산되어 있다.
- 로드맵은 이벤트 기반 무효화 또는 중앙 집중식 정책으로의 전환을 요구한다.
- `Notification`, `PointHistory` 확장을 위해서는 얇은 이벤트 경계가 먼저 필요하다.

관찰

- 기존 원칙은 `repository write 성공이 기준 성공`이며, 캐시 무효화 실패는 write 실패로 승격하지 않는다.
- 현재 코드에서 무효화 책임이 `Board/Post/Comment/Reaction/Attachment` 서비스 전반에 퍼져 있어 누락 리스크가 있다.
- 1차 목표는 outbox/외부 브로커가 아니라 in-process 경계 고정과 캐시 무효화 책임 수렴이다.

결론

- 1차 EDA는 `in-process` 범위에서만 도입한다. outbox/외부 MQ는 포함하지 않는다.
- 이벤트 발행 시점은 `UnitOfWork` 트랜잭션 성공(커밋) 이후로 고정한다.
- 디스패치는 비동기(in-process)로 수행한다.
- 실패 정책은 기존과 동일하게 유지한다.
  - write 성공 우선
  - 이벤트 핸들러 실패/패닉 및 캐시 무효화 실패는 구조화 warn log로만 남긴다.
- 최소 이벤트 타입 세트는 아래 5종으로 고정한다.
  - `BoardChanged` `{operation, boardID}`
  - `PostChanged` `{operation, postID, boardID, tagNames, deletedCommentIDs}`
  - `CommentChanged` `{operation, commentID, postID}`
  - `ReactionChanged` `{operation, targetType, targetID, postID}`
  - `AttachmentChanged` `{operation, attachmentID, postID}`
- operation 값은 `created|updated|deleted|published|set|unset` 중 도메인에 필요한 값만 사용한다.
- `deletedCommentIDs`는 `PostChanged(operation=deleted)`에서만 채운다.

후속 작업

- application port에 `DomainEvent`, `EventPublisher`, `EventBus`, `EventHandler` 계약 추가
- in-process 비동기 event bus 구현 및 panic/error 보호 로깅
- 단일 `CacheInvalidationHandler` 도입 후 서비스 직접 캐시 삭제 제거
- 서비스는 tx 밖에서 이벤트만 발행하도록 전환
- 이벤트/버스/캐시핸들러 및 서비스 회귀 테스트 추가
- 아키텍처 문서에 이벤트 기반 무효화 흐름 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/ARCHITECTURE.md`
- `internal/application/service`

## 2026-03-11 - in-memory 교차 저장소 조회는 coordinator 경계를 우회하지 않는다

상태

- decided

배경

- 최근 lock granularity 개선 후에도 태그 기반 게시글 조회 경로에서 교차 저장소 내부 메서드 직접 호출이 남아 있었다.
- 이 경로는 coordinator 진입을 건너뛰어 tx 락이 의도한 외부 read 차단 의미론을 일부 무력화할 수 있다.
- 동시에 event bus enqueue는 정상 경로에서도 timeout 타이머를 매 호출 생성해 고빈도 publish 비용이 커진다.

관찰

- `PostRepository.selectPublishedPostsByTagName`가 `TagRepository.selectByName`, `PostTagRepository.activePostIDsByTagID`를 직접 호출한다.
- 내부 메서드 호출은 `coordinator.enter()`를 통과하지 않는다.
- `EventBus.Publish`는 queue 여유 상태에서도 `time.After(timeout)`를 생성한다.

결론

- 교차 저장소 조회는 내부 메서드 직접 호출을 지양하고, coordinator 경계를 포함한 public 계약(또는 동등한 query port)을 사용한다.
- event bus publish는 fast-path enqueue를 먼저 시도하고, queue 포화 시에만 timeout 타이머 경로로 진입한다.
- 기존 기능 정책(block + timeout + drop)은 유지한다.

후속 작업

- tag 기반 post 조회 경로를 coordinator 일관 경계로 정리
- tx 중 동시 read 경쟁 회귀 테스트 추가
- publish hot-path의 타이머 생성 회피 테스트 추가
- 관련 단위 테스트/레이스 테스트 재검증

관련 문서/코드

- `internal/infrastructure/persistence/inmemory/postRepository.go`
- `internal/infrastructure/persistence/inmemory/tagRepository.go`
- `internal/infrastructure/persistence/inmemory/postTagRepository.go`
- `internal/infrastructure/event/inprocess/event_bus.go`

## 2026-03-11 - EventBus shutdown 신호 분리 및 태그 조회 경로 읽기 최적화

상태

- decided

배경

- 이벤트 큐 포화 상황에서 publish가 enqueue timeout 대기 중일 때, 종료 시점 동작을 더 명확히 제어할 필요가 있었다.
- 태그 기반 게시글 조회는 coordinator 경계 일관성을 맞추는 과정에서 postTag relation 전체를 clone/sort하는 비용이 커졌다.

관찰

- 기존 구현은 publish 경로가 timeout 기반 대기만 사용해 close 신호와 독립적으로 대기 해제가 어려웠다.
- `SelectPublishedPostsByTagName`는 active relation 전체를 materialize한 뒤 postID set으로 재가공했다.

결론

- EventBus는 `closeCh` 기반 shutdown 신호를 도입해, close 이후 대기 중 publish를 즉시 해제/드롭한다.
- worker는 close 신호 수신 후 queue를 drain하고 종료한다.
- 태그 기반 조회는 `PostTagRepository`의 postID set 전용 read helper를 사용해 clone/sort 경로를 제거한다.
- 두 변경은 기존 기능 정책(큐 포화 시 block+timeout+drop, tx coordinator 경계 유지)을 유지한다.

후속 작업

- close 경계(포화 큐 + 동시 close/publish) 회귀 테스트 유지
- tag relation 대량 데이터 기준 벤치마크 추가 여부 검토

관련 문서/코드

- `internal/infrastructure/event/inprocess/event_bus.go`
- `internal/infrastructure/event/inprocess/bus_test.go`
- `internal/infrastructure/persistence/inmemory/postRepository.go`
- `internal/infrastructure/persistence/inmemory/postTagRepository.go`

## 2026-03-11 - in-process publish를 outbox append + relay로 전환

상태

- decided

배경

- 현재 서비스는 트랜잭션 성공 후 in-process publisher로 즉시 이벤트를 enqueue한다.
- 이 방식은 프로세스 생명주기에 의존해 내구성 있는 비동기 전달 경계로 확장하기 어렵다.
- 로드맵은 outbox 포트 설계 후 SQLite/MQ 어댑터로 확장 가능한 구조를 요구한다.

관찰

- 서비스별 write 유스케이스는 tx 밖에서 `EventPublisher.Publish(...)`를 호출한다.
- UnitOfWork tx scope에는 outbox 저장 경계가 없다.
- 캐시 무효화 소비자는 이미 이벤트 핸들러 형태로 분리되어 있어 relay 소비 경로로 재사용 가능하다.

결론

- 서비스 표준 발행 경로를 `tx 내부 outbox append`로 단일화한다.
- outbox 전달은 in-memory store + relay worker로 1차 구현한다.
- 전달 보장은 at-least-once(재시도/백오프/최대시도 초과 시 dead 상태 보존)로 고정한다.
- 기존 즉시 in-process publish 경로는 제거하고, relay가 이벤트 핸들러를 호출하는 구조로 치환한다.

후속 작업

- application port에 outbox 메시지/저장소/직렬화 포트 추가
- UoW tx scope에 outbox append 경계 추가 및 rollback 결합
- 서비스 이벤트 발행을 tx 내부 append로 이동
- outbox relay worker 구현 및 lifecycle wiring
- 설정/문서/테스트 정합성 반영

관련 문서/코드

- `internal/application/port/*`
- `internal/application/service/*`
- `internal/infrastructure/persistence/inmemory/*`
- `internal/infrastructure/event/*`
- `cmd/main.go`
- `docs/ARCHITECTURE.md`
- `docs/CONFIG.md`

## 2026-03-12 - outbox relay 복구/종료 경계 강화

상태

- decided

배경

- outbox 기반 전환 후에도 worker 비정상 종료 시 `processing` 상태 메시지 복구 규칙이 없어 전달이 정체될 수 있었다.
- dead 메시지 보존 정책과 선형 스캔 polling이 결합되어 메시지 누적 시 relay hot path 비용이 증가한다.
- 서버 종료는 `ListenAndServe` 반환 이후에만 relay stop/wait를 수행해 운영 환경 graceful shutdown 경계가 약했다.

관찰

- `FetchReady`는 `pending`만 대상으로 claim하며, claim 후 상태 복구(lease timeout reclaim)가 없다.
- dead 메시지는 삭제하지 않고 보존하지만, polling 대상 순서에도 남아 반복 스캔된다.
- `main`에는 signal 기반 `http.Server.Shutdown` 경로가 없다.

결론

- outbox claim은 lease timeout 기반 reclaim을 포함해 stale `processing`을 다시 `pending`으로 복구한다.
- dead 메시지는 보존하되 relay polling 순서에서는 분리해 hot path 스캔 비용을 줄인다.
- 서버는 signal 기반 graceful shutdown(`Shutdown -> relay cancel -> relay wait`) 경계를 명시한다.

후속 작업

- outbox reclaim/claim 회귀 테스트 추가
- shutdown 경계 단위 테스트 추가
- ARCHITECTURE/ROADMAP에 운영 경계 업데이트

관련 문서/코드

- `internal/infrastructure/persistence/inmemory/outboxRepository.go`
- `internal/infrastructure/event/outbox/relay.go`
- `cmd/main.go`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`

## 2026-03-12 - dead outbox 메시지 운영 정책과 shutdown bounded wait

상태

- decided

배경

- dead 메시지 보존 정책은 유지하지만, 운영자가 재처리할지 폐기할지 선택하는 명시적 정책이 필요했다.
- signal 기반 graceful shutdown을 도입했지만 server 종료 이벤트 대기에 상한이 없어 장애 시 무기한 대기 가능성이 있었다.

관찰

- dead 메시지는 polling 대상에서 제외되어 성능상 이점이 있지만, 재처리 시 ready 경로 재진입 규칙이 명확하지 않았다.
- `Shutdown` 실패/지연 시 `ListenAndServe` 반환 대기가 길어질 수 있다.

결론

- dead 메시지는 기본 보존한다.
- 운영자 처리 정책은 다음으로 고정한다.
  - 재처리: `dead -> pending(ready)` 전환
  - 폐기: 영구 삭제(discard)
- 구현은 기존 store 계약 내에서 아래로 매핑한다.
  - 재처리: `MarkRetry` 호출 시 dead 메시지도 ready 순서에 재삽입
  - 폐기: `MarkSucceeded`로 제거
- shutdown 경계는 bounded wait + fallback close로 고정한다.

후속 작업

- dead->pending 재큐잉 회귀 테스트 추가
- signal shutdown timeout/fallback close 테스트 추가
- 운영자 재처리/폐기 API는 다음 단계에서 추가

관련 문서/코드

- `internal/infrastructure/persistence/inmemory/outboxRepository.go`
- `cmd/main.go`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`

## 2026-03-12 - 이벤트 발행 경계를 action dispatch 용어로 정리

상태

- decided

배경

- outbox 경로가 표준이 된 이후에도 서비스 계층의 `eventPublisher` 명칭은 향후 action/filter hook 확장 의도를 드러내지 못했다.
- 동시에 일부 경계에서는 tx outbox가 없는 환경(테스트/확장 어댑터)에서 이벤트 전달 fallback 전략이 필요했다.

관찰

- 서비스 write 경로는 `tx.Outbox().Append`를 사용하지만 필드/생성자 이름은 여전히 `eventPublisher` 중심이다.
- action hook 확장 로드맵 관점에서는 "이벤트 퍼블리셔"보다 "도메인 액션 디스패치" 용어가 의도에 가깝다.

결론

- 서비스 계층 용어를 `actionDispatcher`로 정리한다.
- 공통 helper는 `dispatchDomainActions(tx, dispatcher, events...)`로 통일한다.
- dispatch 규칙은 다음으로 고정한다.
  - tx outbox가 있으면 outbox append 우선
  - outbox가 없으면 dispatcher fallback publish

후속 작업

- 다음 단계에서 filter/action hook 포트 구체화
- 서비스 생성자 naming(`WithPublisher`)은 호환성 고려 후 단계적으로 정리

관련 문서/코드

- `internal/application/service/*Service.go`
- `internal/application/service/outbox_events.go`

## 2026-03-13 - PR 리뷰 반영: 테스트 relay 수명/종료 경계/Outbox 검증 보강

상태

- decided

배경

- PR 리뷰에서 테스트 헬퍼 relay의 무기한 goroutine 생존 가능성, graceful shutdown 경계의 defer 순서/시간 예산 이중 사용, outbox 설정 음수 케이스 검증 누락이 지적되었다.

관찰

- `internal/application/service/common_test.go` 헬퍼는 `context.Background()`로 relay를 시작해 테스트 종료 시 정리 훅이 없었다.
- `cmd/main.go` graceful shutdown은 `defer` 순서상 `relay.Wait()`가 `cancel()`보다 먼저 실행될 수 있고, timeout budget을 단계별로 재사용해 총 대기 시간이 늘어날 수 있다.
- `internal/config/config_test.go`는 outbox 정상값 검증은 있으나 각 필드별 invalid 케이스 회귀 테스트가 부족하다.

결론

- 테스트 relay는 `testing.TB` 기반 helper에서 `context.WithCancel` + `t.Cleanup(cancel+relay.Wait)`로 수명을 테스트 스코프로 제한한다.
- graceful shutdown은 defer 의존을 제거하고 단일 deadline 기반 timeout budget으로 서버 종료/강제 종료 경계를 일원화한다.
- outbox 설정은 `workerCount/batchSize/pollIntervalMillis/maxAttempts/baseBackoffMillis` 각 필드의 invalid 케이스를 table-driven 테스트로 보강한다.

후속 작업

- 필요 시 `cmd/main.go` fallback wait 상수를 설정값으로 승격 검토
- 테스트 helper naming(`WithPublisher`) 정리 시 `WithActionDispatcher` 용어 통일 추가 검토

관련 문서/코드

- `internal/application/service/common_test.go`
- `cmd/main.go`
- `cmd/main_test.go`
- `internal/config/config_test.go`

## 2026-03-13 - PR 후속 리뷰 반영: 이벤트 디스패치 분기 정리와 serializer 중복 제거

상태

- decided

배경

- `1cf2793` 이후 PR 리뷰에서 `dispatchDomainActions`의 중복 조건 분기와 `JSONEventSerializer.Deserialize`의 반복 코드가 지적되었다.

관찰

- `dispatchDomainActions`는 함수 시작에서 `len(events)==0` 반환 후, 다음 분기에서 동일 조건을 다시 검사한다.
- `Deserialize`는 이벤트 타입별로 동일한 `unmarshal -> At zero이면 occurredAt 보정` 로직을 반복한다.

결론

- `dispatchDomainActions`는 중복 조건을 제거하고 의도(이벤트 없음/tx 없음/outbox 없음)를 단일 분기로 유지한다.
- `Deserialize`는 공통 역직렬화 helper로 중복을 제거하되 이벤트 이름별 타입 매핑과 unsupported 에러 동작은 유지한다.
- serializer 동작 회귀 방지를 위해 전용 단위 테스트를 추가한다.

후속 작업

- 이벤트 타입이 추가될 때 serializer 테스트 케이스를 함께 확장한다.

관련 문서/코드

- `internal/application/service/outbox_events.go`
- `internal/application/event/serializer.go`
- `internal/application/event/serializer_test.go`

## 2026-03-13 - Background Delivery 계층, `slog` 직접 주입, `context.Context` 전파 원칙을 아키텍처 표준으로 고정

상태

- decided

배경

- 아키텍처 문서는 HTTP Delivery 경계는 분명히 설명하지만, outbox relay worker와 attachment cleanup job 같은 백그라운드 실행 주체가 어느 레이어에 속하는지 명시하지 않았다.
- 문서에는 application이 로깅을 port로 호출한다고 적혀 있었지만, 실제 운영 로깅 요구와 표준 라이브러리 사용 관점에서 더 단순한 원칙이 필요했다.
- `UnitOfWork`, outbox, graceful shutdown, 향후 tracing을 일관되게 연결하려면 delivery에서 시작된 `context.Context`를 끝단까지 전파하는 규칙이 문서와 코드 모두에 필요했다.

관찰

- 현재 `cmd/main.go`는 HTTP 서버, outbox relay, background job runner를 함께 wiring하는 composition root 역할을 수행한다.
- background runner/job은 이미 use case를 호출하는 형태로 연결돼 있으나, 이 구조가 아키텍처 규칙으로 고정돼 있지 않았다.
- 일부 포트(`UnitOfWork`)는 아직 `context.Context`를 받지 않아 전파 원칙과 구현이 완전히 맞물리지 않았다.

결론

- 백그라운드 워커, 주기 잡, 이벤트 컨슈머는 모두 HTTP와 동급의 `Primary Adapter`, 즉 `Delivery` 계층으로 본다.
- Delivery는 `HTTP Delivery`와 `Background Delivery`로 나눈다.
- `Background Delivery` 책임은 polling, schedule trigger, retry/ack, shutdown 경계 관리와 입력 해석까지만 가진다.
- 실제 비즈니스 규칙, 권한 판정, tx 경계, outbox append 같은 변경 로직은 `UseCase Port`와 `Application Service`에서 수행한다.
- 워커/컨슈머가 repository 또는 DB 구현체를 직접 호출하는 것은 금지한다.
- 로깅은 도메인 포트로 취급하지 않는다.
- 애플리케이션/인프라 구현은 필요 시 표준 `*slog.Logger`를 DI로 주입받아 사용한다.
- composition root는 HTTP 예외 처리, relay, background job이 공유할 로거 인스턴스를 생성해 전달한다.
- 모든 Application Port, UseCase, Repository, Infrastructure Adapter 메서드는 `context.Context`를 첫 번째 인자로 받는 것을 기본 원칙으로 삼는다.
- `UnitOfWork` 역시 `WithinTransaction(ctx, fn)` 형태로 상위 컨텍스트를 전달받아 같은 요청/작업 범위의 취소 신호와 추적 문맥을 유지한다.
- `context.WithValue`는 delivery/middleware 같은 경계에서만 제한적으로 사용하고, application/domain 내부에서 임의 request-scoped 값을 추가하지 않는다.
- outbox relay는 도메인 쓰기 use case가 아니라 outbox를 전달/소비하는 background delivery adapter로 문서화한다.

후속 작업

- `docs/ARCHITECTURE.md`에 background delivery, logging, context propagation 규칙 반영
- `port.Logger` 계층 제거 및 `*slog.Logger` 직접 주입으로 코드 정합화
- `UnitOfWork`와 서비스 경계의 `context.Context` 시그니처 정리 및 회귀 테스트 보강

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `cmd/main.go`
- `internal/delivery/http.go`
- `internal/application/port/unit_of_work.go`
- `internal/infrastructure/event/outbox/relay.go`
- `internal/infrastructure/job/inprocess/runner.go`

## 2026-03-13 - Repository, Cache, SessionRepository, FileStorage 포트도 `context.Context`를 첫 번째 인자로 통일한다

상태

- decided

배경

- 이전 단계에서 Delivery, UseCase, Service, UnitOfWork 경계에 `context.Context`를 올렸지만, 하위 repository/cache/storage 포트는 여전히 일부 메서드가 `ctx` 없이 동작했다.
- 이 상태에서는 요청 취소, deadline, tracing, background job cancellation이 service 아래 계층으로 완전히 전파되지 않는다.

관찰

- 현재 서비스는 `UnitOfWork.WithinTransaction(ctx, fn)`까지는 동일한 상위 context를 전달한다.
- 하지만 tx 안팎의 repository/cache/file storage 호출은 `ctx`를 받지 않아 실제 I/O 경계에서 request scope를 활용할 수 없다.
- `SessionRepository`는 cache adapter 위에 놓여 있으므로 함께 정리하지 않으면 인증 경로의 context 일관성이 깨진다.

결론

- 모든 Repository 포트(`User/Board/Post/Tag/PostTag/Comment/Reaction/Attachment`)는 `context.Context`를 첫 번째 인자로 받는다.
- `Cache` 포트의 `Get/Set/SetWithTTL/Delete/DeleteByPrefix/GetOrSetWithTTL`도 `context.Context`를 첫 번째 인자로 받는다.
- `GetOrSetWithTTL`의 loader 역시 `func(ctx context.Context) (interface{}, error)` 형태로 상위 context를 그대로 받는다.
- `SessionRepository`와 `FileStorage` 포트도 동일하게 `context.Context`를 첫 번째 인자로 받는다.
- 서비스는 tx 밖에서는 자신이 받은 `ctx`를, tx 안에서는 `tx.Context()`를 사용해 하위 포트를 호출한다.
- `TokenProvider`, `PasswordHasher`처럼 순수 계산/포맷팅 성격의 포트는 이번 단계에서 `ctx` 대상에 포함하지 않는다.

후속 작업

- 포트/어댑터/테스트 시그니처 정리
- 서비스와 delivery에서 tx 안팎의 repository/cache/storage 호출을 `ctx` 기반으로 전환
- 전체 회귀 테스트로 cancellation/loader 전달 경계 확인

관련 문서/코드

- `internal/application/port/*_repository.go`
- `internal/application/port/cache.go`
- `internal/application/port/session_repository.go`
- `internal/application/port/file_storage.go`
- `internal/application/service/*.go`
- `internal/infrastructure/persistence/inmemory`
- `internal/infrastructure/cache`
- `internal/infrastructure/auth/CacheSessionRepository.go`
- `internal/infrastructure/storage`

## 2026-03-13 - 인증 credential 검증에도 request context를 전달하고 JWT secret 최소 강도 정책을 추가한다

상태

- decided

배경

- 로그인 경로는 `SessionService.Login(ctx, ...)`로 시작하지만, credential 검증 포트는 `ctx` 없는 시그니처라 사용자 조회 경계에서 cancellation/deadline/trace 문맥이 끊겼다.
- JWT secret 검증은 empty/placeholder만 차단하고 최소 길이 기준이 없어 운영 실수로 약한 키가 배포될 여지가 있었다.
- 서비스 계층에 미사용 helper가 남아 있어 코드 간결성과 경계 규칙(context 일관성) 관점에서 정리가 필요했다.

관찰

- `CredentialVerifier.VerifyCredentials(username, password)`는 `ctx`를 받지 않는다.
- `UserService.VerifyCredentials`는 내부에서 `context.Background()`를 사용한다.
- `delivery.http.auth.secret`은 trim/placeholder 검증은 있으나 길이 기준이 없다.
- `cache_invalidation.go`, `postService.go` 일부 helper는 호출 경로가 없다.

결론

- `CredentialVerifier` 포트 시그니처를 `VerifyCredentials(ctx context.Context, username, password)`로 변경한다.
- `SessionService.Login`은 상위 request `ctx`를 credential verifier까지 전달한다.
- `UserService.VerifyCredentials`는 전달받은 `ctx`로 repository를 호출한다.
- JWT secret은 trim 후 최소 길이(32자 이상) 정책을 적용한다.
- 미사용 helper(`cache_invalidation.go`의 best-effort 함수, `PostService`의 미사용 non-tx tag helper)는 제거해 코드와 설계 규칙을 정합화한다.

후속 작업

- 포트 시그니처 변경에 따른 서비스/테스트 업데이트
- config validation 및 테스트에 최소 길이 정책 반영
- CONFIG 문서에 JWT secret 최소 강도 기준 명시

관련 문서/코드

- `internal/application/port/credential_verifier.go`
- `internal/application/service/sessionService.go`
- `internal/application/service/userService.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `docs/CONFIG.md`

## 2026-03-13 - JWT claim 타입 안정성과 이벤트 핸들러 context 전파를 정렬한다

상태

- decided

배경

- JWT `user_id`를 `MapClaims`에서 `float64`로 읽는 방식은 큰 정수 식별자에서 정밀도 손실 위험이 있다.
- 아키텍처 원칙은 경계 전반의 `context.Context` 전파를 요구하지만, 이벤트 핸들러 포트는 context를 받지 않아 relay 취소/종료 문맥이 하위 I/O 경계까지 전달되지 않는다.

관찰

- `JwtTokenProvider.ValidateTokenToId`는 `claims["user_id"].(float64)` 캐스팅을 사용한다.
- `EventHandler` 포트는 `Handle(event DomainEvent)` 시그니처다.
- outbox relay와 in-process bus는 handler 호출 시 context를 전달하지 않는다.
- cache invalidation handler는 내부에서 `context.Background()`를 사용한다.

결론

- JWT claims를 typed struct로 전환해 `user_id int64`를 타입 안정적으로 직렬화/역직렬화한다.
- 이벤트 핸들러 포트를 `Handle(ctx context.Context, event DomainEvent)`로 확장한다.
- outbox relay/in-process bus는 worker/request context를 handler까지 전달한다.
- cache invalidation handler는 전달받은 ctx를 cache 포트 호출에 그대로 전달한다.

후속 작업

- JWT large-id round-trip 회귀 테스트 추가
- 이벤트 핸들러 context 전달/취소 경계 테스트 추가
- 아키텍처 문서의 이벤트 경계 문구를 새 포트 시그니처에 맞춰 정합화

관련 문서/코드

- `internal/infrastructure/auth/JwtTokenProvider.go`
- `internal/application/port/event.go`
- `internal/infrastructure/event/outbox/relay.go`
- `internal/infrastructure/event/inprocess/event_bus.go`
- `internal/application/event/cache_invalidation_handler.go`
- `docs/ARCHITECTURE.md`

## 2026-03-14 - 미사용 in-process EventBus를 제거하고 태그 조회 경로 context 전파를 정렬한다

상태

- decided

배경

- 현재 production wiring은 outbox relay만 사용하고 in-process EventBus 어댑터는 테스트 외 사용 경로가 없다.
- 미사용 어댑터를 유지하면 별도 lifecycle/backpressure/context 정책을 지속 관리해야 해 코드 복잡도와 정책 드리프트 위험이 커진다.
- `PostRepository.SelectPublishedPostsByTagName` 내부가 `context.Background()`를 고정 사용해 상위 request context 불변성 원칙과 충돌한다.

관찰

- `cmd/main.go`는 outbox relay 구독/시작만 수행하고 in-process EventBus는 조립하지 않는다.
- `internal/application/port/event.go`의 `EventBus` 계약은 현재 비테스트 경로에서 실사용되지 않는다.
- `internal/infrastructure/persistence/inmemory/postRepository.go`의 태그 조회 내부 호출은 전달된 `ctx` 대신 `context.Background()`를 사용한다.

결론

- `internal/infrastructure/event/inprocess` 패키지를 제거한다.
- 미사용 `EventBus` 포트 계약을 제거하고, 이벤트 발행 계약은 `EventPublisher` + `EventHandler`로 단순화한다.
- `SelectPublishedPostsByTagName` 경로는 상위 `ctx`를 내부 태그 조회까지 그대로 전달한다.

후속 작업

- 태그 조회 경로 context 전달 회귀 테스트 추가
- 전체 테스트 스위트 실행으로 제거 영향 확인

관련 문서/코드

- `internal/infrastructure/event/inprocess`
- `internal/application/port/event.go`
- `internal/infrastructure/persistence/inmemory/postRepository.go`
- `internal/infrastructure/persistence/inmemory/unitOfWork.go`

## 2026-03-14 - 서비스 생성자 경계를 ActionDispatcher 단일 경로로 정리한다

상태

- decided

배경

- 운영 경로는 outbox 기반 `ActionHookDispatcher`를 사용하지만, 서비스 API에는 `New*WithPublisher` 생성자가 병행 유지되고 있었다.
- `resolveEventPublisher`/`noopEventPublisher`는 deprecated 상태로 남아 있으나 실제 비테스트 경로에서 사용되지 않는다.

관찰

- `cmd/main.go`는 `New*WithActionDispatcher(..., nil, ...)` 경로를 사용한다.
- 서비스 테스트 다수는 여전히 `New*WithPublisher` + `newTestEventPublisher` 헬퍼를 사용한다.
- `event_publisher.go`의 deprecated helper는 참조가 없다.

결론

- 서비스 생성자 표준 경계를 `New*WithActionDispatcher`로 단일화한다.
- `New*WithPublisher` 생성자와 미사용 deprecated helper(`resolveEventPublisher`, `noopEventPublisher`)를 제거한다.
- 테스트 헬퍼도 `port.ActionHookDispatcher` 기반으로 정렬한다.

후속 작업

- 서비스/테스트 컴파일 경계를 action dispatcher 기준으로 일괄 갱신
- 전체 테스트 및 race/vet로 회귀 확인

관련 문서/코드

- `internal/application/service/*Service.go`
- `internal/application/service/common_test.go`
- `internal/application/service/event_publisher.go`

## 2026-03-14 - Report 도메인과 admin 운영 API(신고/Dead Outbox/게시판 hidden)를 1차 도입한다

상태

- decided

배경

- Step 2 미반영 범위인 `Report` 도메인을 도입해야 한다.
- 운영 API는 현재 suspension/board CRUD 중심이라 신고 운영과 dead outbox 수동 처리 경로가 비어 있다.
- 게시판 운영 관점에서 비노출(hidden) 제어가 필요하다.

관찰

- `docs/ROADMAP.md`는 `Report`와 dead 이벤트 운영 재처리(admin/ops) 경로를 다음 범위로 명시한다.
- 현재 outbox 저장소는 `MarkRetry/MarkDead/MarkSucceeded`는 제공하지만 dead 목록 조회 read 계약은 없다.
- 게시판 엔티티/저장소에는 hidden 상태가 없다.

결론

- 신고 대상은 `post`, `comment`로 제한한다.
- 신고 상태는 `pending`, `accepted`, `rejected`로 두고 처리 시 자동 제재/자동 숨김은 하지 않는다.
- 동일 `(reporter_user_id, target_type, target_id)` 중복 신고는 금지한다.
- 현재 공개 API 입력은 `target_uuid`를 받지만, 내부 무결성 제약은 대상의 내부 식별자 기준으로 유지한다.
- 신고 사유는 `enum + detail text`로 받는다.
- admin 신고 목록은 `pending` 우선 + 최신순으로 제공한다.
- dead outbox 운영 API는 `목록 + 재처리(requeue) + 폐기(discard)`를 제공한다.
- 게시판 운영 확장은 `hidden`만 도입하고, hidden 게시판은 비admin에서 완전 비노출로 처리한다.
- admin 조치(신고 처리, dead outbox 조치, hidden 변경)는 DB 감사로그 대신 구조화 로그로 시작한다.

후속 작업

- Report entity/repository/usecase/service + HTTP API 추가
- OutboxStore에 dead 목록 조회 포트 추가 및 admin usecase/API 연결
- Board hidden 상태 및 admin visibility API 추가
- event 타입에 `report.changed` 추가 및 serializer/relay 구독 반영
- 테스트(TDD) 보강 후 API/ARCHITECTURE/ROADMAP/Swagger 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `internal/domain/entity`
- `internal/application/port`
- `internal/application/service`
- `internal/delivery/http.go`
- `internal/infrastructure/persistence/inmemory`

## 2026-03-14 - 태그 기반 게시글 목록에도 hidden 게시판 비노출 정책을 일관 적용한다

상태

- decided

배경

- hidden 게시판은 비admin에게 완전 비노출이 정책이지만, 태그 기반 게시글 목록 경로는 동일 정책을 서비스 경계에서 강제하지 못했다.
- 이로 인해 hidden 게시판 게시글이 태그 목록에 노출될 수 있어 정책 일관성과 보안 기대치에 어긋난다.

관찰

- `GetPostsList`, `GetPostDetail` 등은 `policy.EnsureBoardVisible`을 사용한다.
- `GetPostsByTag`는 게시글-태그 관계만으로 목록을 만들고 board visibility 검증을 수행하지 않는다.

결론

- 태그 목록 경로는 별도 hidden 필터를 추가하지 않고, 기존 `policy.EnsureBoardVisible`을 재사용해 정책 단일 경계를 유지한다.
- 당시 구현 시 hidden 게시판 게시글을 건너뛰는 동안에도 커서 pagination(`has_more`, `next_last_id`) 의미를 보존하도록 visibility-aware 조회를 적용했다.
- 현재 공개 목록 계약은 opaque `cursor`, `next_cursor`를 사용한다.

후속 작업

- `PostService.GetPostsByTag` 경로에 visibility-aware 페이징 로직 추가
- hidden 게시판 게시글 비노출 및 커서 정합성 테스트(TDD) 추가

관련 문서/코드

- `internal/application/policy/board_visibility_policy.go`
- `internal/application/service/postService.go`
- `internal/application/service/postService_test.go`

## 2026-03-14 - Round 25 잔여 항목(R25-02/04/05)을 운영 추적성과 코드 일관성 중심으로 반영한다

상태

- decided

배경

- Round 25에서 남아 있던 잔여 항목은 업로드 롤백 실패 운영 추적성, error 패키지 경로 관례 불일치, 의미 없는 구현 주석 정리였다.

관찰

- 첨부 업로드에서 메타 저장 실패 후 스토리지 롤백이 실패하면 운영자가 즉시 원인을 추적하기 어려웠다.
- `internal/customError` 디렉토리와 `package customerror` 명칭이 불일치했다.
- board/comment 서비스에 스캐폴딩성 주석이 남아 코드 가독성을 저해했다.

결론

- 첨부 업로드 롤백 실패 시 `storage_key`를 포함한 warn 로그를 남긴다.
- error 패키지 디렉토리를 `internal/customerror`로 정렬하고 import 경로를 일괄 갱신한다.
- 구현 설명성 주석(`게시판/댓글 ... 로직 구현`)은 제거한다.

후속 작업

- 스토리지-DB 보상 전략(2-phase 또는 outbox 기반) 고도화는 별도 설계 항목으로 유지한다.

관련 문서/코드

- `internal/application/service/attachmentService.go`
- `internal/application/service/attachmentService_test.go`
- `internal/customerror`
- `internal/application/service/boardService.go`
- `internal/application/service/commentService.go`

## 2026-03-14 - 태그 목록 hidden 필터 보완 후 리뷰 반영으로 페이징 overflow 방어와 배치 보드 조회를 추가한다

상태

- decided

배경

- 태그 목록 visibility-aware 구현 이후 코드리뷰에서 `limit+1` overflow panic 가능성과 보드 조회 N+1 가능성이 지적됐다.

관찰

- 공개 API의 `limit`은 하한(1)만 검증하고 상한은 서비스 레이어에 위임된다.
- `loadPublishedPostsByTag`는 hidden 게시판 차단을 위해 게시글별 보드 조회를 수행하므로, 게시글이 여러 보드에 분산되면 호출 수가 증가할 수 있다.

결론

- 커서 조회용 `limit+1` 계산은 overflow-safe helper를 통해 수행한다.
- 태그 목록 visibility 판단 시 배치 단위로 보드를 조회하는 `SelectBoardsByIDs` 포트를 추가해 조회 횟수를 줄인다.
- visibility bool 계산은 `policy.EnsureBoardVisible(board, nil) == nil` 형태로 단순화한다.

후속 작업

- `BoardRepository`/in-memory 구현/contract test에 `SelectBoardsByIDs` 추가
- `PostService` 태그 목록 경로에 overflow-safe 페이징 + 배치 보드 조회 적용
- 관련 회귀 테스트(대형 limit 입력) 보강

관련 문서/코드

- `internal/application/service/pagination.go`
- `internal/application/port/board_repository.go`
- `internal/infrastructure/persistence/inmemory/boardRepository.go`
- `internal/application/service/postService.go`
- `internal/application/service/postService_test.go`

## 2026-03-14 - 목록 API limit 상한을 도입해 과대 할당 기반 DoS 가능성을 차단한다

상태

- decided

배경

- `limit+1` overflow는 차단했지만, 매우 큰 `limit` 자체는 대규모 메모리 할당 시도를 유발할 수 있다.
- 목록 API 전반은 `requirePositiveLimit`를 공유하므로 상한을 공통 적용하는 것이 경계 일관성과 운영 안전성에 유리하다.

관찰

- `GetBoards`, `GetPostsList`, `GetPostsByTag`, `GetCommentsByPost`, `GetReports`, `GetDeadMessages`가 동일 검증 함수를 사용한다.
- 상한이 없으면 서비스/저장소 레이어에서 불필요하게 큰 slice capacity 또는 대량 조회를 시도할 수 있다.

결론

- 공통 pagination limit 상한을 `1000`으로 고정한다.
- `limit < 1` 또는 `limit > 1000`은 `ErrInvalidInput`으로 처리한다.
- 기존 overflow 방어는 유지해 방어 코드를 중첩 적용한다.

후속 작업

- `requirePositiveLimit`/`cursorFetchLimit` 경계 테스트 보강
- 태그 목록 경로 상한 검증 테스트 보강

관련 문서/코드

- `internal/application/service/pagination.go`
- `internal/application/service/pagination_test.go`
- `internal/application/service/postService_test.go`

## 2026-03-14 - pagination 상한 도입 이후 도달 불가 overflow 분기를 제거한다

상태

- decided

배경

- `maxPageLimit` 상한 도입으로 `cursorFetchLimit` 입력은 항상 제한된다.

관찰

- `cursorFetchLimit` 내부 `limit > math.MaxInt-1` 체크는 상한(`1000`)보다 훨씬 큰 값이어서 도달 불가능하다.

결론

- 도달 불가 분기를 제거해 pagination 검증 코드를 단순화한다.
- 입력 검증 책임은 `requirePositiveLimit`에 유지한다.

후속 작업

- `cursorFetchLimit` 구현 단순화
- pagination 경계 테스트 통과 확인

관련 문서/코드

- `internal/application/service/pagination.go`
- `internal/application/service/pagination_test.go`

## 2026-03-14 - limit 계약/secret 강도/hidden 책임을 최신 정책으로 정렬한다

상태

- decided

배경

- pagination 상한이 코드에 도입됐지만 공개 API 문서/Swagger에는 상한 정보가 누락되어 있었다.
- JWT secret 최소 길이는 결정 기록(32자)과 실제 검증(16자)이 어긋나 있었다.
- board hidden 필터가 service와 repository에 중복되어 계층 책임이 불명확했다.

관찰

- 서비스 공통 검증은 `limit > 1000`을 거부한다.
- `internal/config/config.go`는 `minJWTSecretLength = 16`으로 동작했다.
- in-memory `SelectBoardList`와 `BoardService.GetBoards`가 모두 hidden 필터를 수행했다.

결론

- 목록 API의 `limit` 계약을 `1..1000`으로 명시하고 delivery 파서/Swagger/문서를 일치시킨다.
- JWT secret 최소 길이를 32자로 상향해 기존 결정과 구현을 정렬한다.
- hidden 비노출 판단 책임은 policy+service 레이어로 단일화하고 repository는 원시 데이터 조회 책임으로 유지한다.

후속 작업

- `parseLimitLastID`/`parseLimitLastIDString` 상한 검증 추가
- Swagger/`docs/API.md`의 limit 파라미터 상한 반영
- config 검증/테스트 secret 길이 기준 상향
- board repository hidden 필터 제거 및 service policy 적용 유지

관련 문서/코드

- `internal/delivery/http.go`
- `docs/API.md`
- `docs/swagger`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `docs/CONFIG.md`
- `internal/application/service/boardService.go`
- `internal/infrastructure/persistence/inmemory/boardRepository.go`

## 2026-03-14 - 보드 목록은 반복조회 대신 조회계층에서 visible 커서를 보장하고 페이지 기본값만 설정화한다

상태

- decided

배경

- hidden 보드가 상위 ID 구간을 차지하면, 서비스 후처리 필터 방식은 `has_more` 정합성이 깨질 수 있다.
- 운영 튜닝 요구가 있으나, 안전 상한(`max page limit`)은 보안 경계이므로 런타임 설정으로 열지 않는 것이 안전하다.

관찰

- 보드 목록은 `fetchLimit=limit+1` 조회 후 hidden 후처리를 수행한다.
- 페이지 상한은 서비스/딜리버리에서 `1000`으로 고정되어 있으며 DoS 방어 경계 역할을 한다.

결론

- 보드 공개 목록은 조회 계층에서 hidden 제외 + 커서 적용을 수행해 반복조회 없이 정합성을 보장한다.
- 페이지 설정은 `default page limit`만 config로 노출하고, `max page limit(1000)`은 코드 상수로 유지한다.

후속 작업

- board list hidden+cursor 정합성 회귀 테스트 추가
- `delivery.http.defaultPageLimit` 설정 추가 및 검증(`1..1000`)
- HTTP 파서 기본값을 설정값으로 주입

관련 문서/코드

- `internal/application/service/boardService.go`
- `internal/infrastructure/persistence/inmemory/boardRepository.go`
- `internal/delivery/http.go`
- `internal/config/config.go`

## 2026-03-14 - Step 3 첫 구현으로 IP 기반 쓰기 요청 Rate Limit을 도입한다

상태

- decided

배경

- 로드맵 Step 3(어뷰징 방지/보안)에서 도배성 쓰기 요청을 제어할 최소 안전장치가 필요하다.
- 현재 HTTP 경계에는 바디 크기 제한은 있으나, 동일 클라이언트의 고빈도 쓰기 요청을 제어하는 장치가 없다.

관찰

- 인증 미들웨어는 라우트별로 적용되어 있어, 전역 요청 제어는 API 경계(`v1`)에서 처리하는 편이 일관적이다.
- 쓰기 요청 경계는 HTTP 메서드(`POST/PUT/DELETE/PATCH`)로 명확히 분리 가능하다.

결론

- `RateLimiter` 포트를 추가하고 기본 어댑터는 in-memory fixed-window 카운터로 시작한다.
- 적용 범위는 `/api/v1` 하위 쓰기 메서드 전체로 제한하고, 읽기 메서드(`GET/HEAD/OPTIONS`)는 제외한다.
- 키 전략은 `method + route + client_ip` 조합으로 고정한다.
- 초과 시 공개 에러는 `too many requests`로 응답하고 상태코드는 `429`를 사용한다.
- 설정은 `delivery.http.rateLimit.{enabled,windowSeconds,writeRequests}`로 주입한다.

후속 작업

- HTTP 통합 테스트로 `429` 회귀 케이스를 추가한다.
- config 검증/문서/샘플 설정/Swagger를 정책과 일치시킨다.
- 추후 persistent 또는 분산 limiter로 교체 가능한 포트 경계를 유지한다.

관련 문서/코드

- `internal/application/port`
- `internal/infrastructure/ratelimit/inmemory`
- `internal/delivery/http.go`
- `internal/config/config.go`
- `docs/CONFIG.md`

## 2026-03-14 - RF-54/RF-55를 반영해 admin HTTP 경계를 강화하고 post 삭제 cascade를 배치화한다

상태

- decided

배경

- Round 25 리뷰에서 admin 라우트의 HTTP 방어 심도 부족과 post 삭제 경로의 대량 댓글 메모리 사용 위험이 지적됐다.

관찰

- 기존 `/api/v1/admin/**`는 인증 미들웨어만 걸려 있었고, 역할 검증은 서비스 계층에만 위임됐다.
- `PostService.DeletePost`는 댓글 cascade를 위해 `math.MaxInt` 수준의 단일 조회를 사용해 대량 댓글 포스트에서 OOM 리스크가 있었다.

결론

- admin 라우트 그룹에 역할 미들웨어를 추가해 비admin 요청을 HTTP 레이어에서 즉시 `403`으로 차단한다.
- 서비스 레이어의 `AdminOnly` 검증은 defense-in-depth를 위해 유지한다.
- post 삭제의 댓글 cascade는 고정 배치 크기(`postDeleteBatchSize`) 반복 처리로 변경해 메모리 사용 상한을 `O(batch)`로 제한한다.

후속 작업

- 대규모 삭제 시 처리시간/락 점유를 줄이기 위한 저장소 bulk-delete contract 도입 여부를 별도 검토한다.
- admin 라우트 추가 시 미들웨어 경계를 우회하지 않도록 라우팅 규칙을 리뷰 체크리스트에 포함한다.

관련 문서/코드

- `internal/delivery/middleware/adminRoleMiddleware.go`
- `internal/delivery/http.go`
- `internal/application/service/postService.go`
- `internal/application/service/postService_test.go`

## 2026-03-14 - Step 3의 XSS 방어는 입력 경계에서 HTML escape sanitizer 파이프라인으로 시작한다

상태

- decided

배경

- Rate limit 도입 이후에도 저장형 XSS를 줄이기 위한 입력 정제가 필요하다.
- 현재 게시판/게시글/댓글/신고 텍스트 입력은 별도 sanitizer 없이 그대로 저장될 수 있다.

관찰

- 본 서비스는 markdown 기반 텍스트를 주로 다루며, HTML raw input 허용이 필수 요구사항으로 고정되어 있지 않다.
- delivery 계층은 요청 파싱과 경계 검증 책임을 이미 가지므로 sanitizer 파이프라인의 첫 적용 지점으로 적합하다.

결론

- `InputSanitizer` 포트를 추가하고 기본 구현은 HTML escape 전략으로 시작한다.
- `/api/v1` 쓰기 핸들러에서 게시판/게시글/댓글/신고 관련 텍스트를 usecase 전달 전에 sanitize한다.
- 설정 키 `delivery.http.sanitizer.enabled`로 on/off 제어를 제공하고 기본값은 `true`로 둔다.

후속 작업

- sanitizer 어댑터를 policy 기반 allowlist(예: markdown-safe subset)로 확장할지 검토한다.
- 추후 non-HTTP ingress(consumer/batch)에서도 동일 sanitizer policy를 재사용할 수 있게 경계를 정리한다.

관련 문서/코드

- `internal/application/port/input_sanitizer.go`
- `internal/infrastructure/sanitizer/escape`
- `internal/delivery/http.go`
- `internal/config/config.go`
- `docs/CONFIG.md`

## 2026-03-15 - 공개 리소스 식별자는 숫자 ID 대신 UUID를 사용하고 목록 커서는 opaque cursor로 전환한다

상태

- decided

배경

- 현재 외부 API는 `User`만 UUID를 공개 식별자로 사용하고, `Board/Post/Comment/Attachment`는 숫자 ID를 그대로 노출한다.
- 개발 단계이므로 하위호환 유지 비용 없이 외부 계약을 정리할 수 있다.
- 목록 API의 `last_id`는 내부 정렬 키를 외부에 노출하므로, 공개 식별자 전환과 함께 경계를 정리할 필요가 있다.

관찰

- 내부 저장/연관/정렬은 숫자 PK/FK에 강하게 묶여 있으며, 트랜잭션과 저장소 구현도 이를 전제로 한다.
- 공개 경계는 path param, 요청 본문(`report target_id`), 응답 payload, attachment URL, cursor query에 걸쳐 숫자 ID를 사용한다.
- 운영용 리소스(`reportID`, `messageID`)는 공개 URL 가독성보다 운영 추적성이 중요하다.

결론

- `Board`, `Post`, `Comment`, `Attachment`는 생성 시 공개 UUID를 부여하고, 외부 API는 UUID만 사용한다.
- 내부 저장의 주키/외래키는 계속 숫자 ID를 유지하고, 서비스가 UUID 조회 후 내부 ID로 기존 비즈니스 로직을 수행한다.
- 공개 목록 API는 `last_id`를 제거하고 opaque `cursor`를 사용한다. 현재 정렬 의미는 유지하되 내부 정렬 키는 인코딩해 외부에 직접 노출하지 않는다.
- admin 게시판 visibility API도 `boardUUID`로 통일한다.
- 운영 리소스 식별자인 `reportID`, `messageID`는 이번 배치에서 유지한다.
- 공개 응답의 리소스 기본 식별자는 `uuid`로 통일하고, 연관 표기는 기존 관례대로 `author_uuid`, `board_uuid`, `post_uuid` 등을 사용한다.

후속 작업

- 저장소 contract에 UUID 조회/유니크 규약을 추가한다.
- HTTP/Swagger/API 문서에서 숫자 ID path/body/query 계약을 UUID/cursor 계약으로 일괄 교체한다.
- report 생성은 `target_uuid` 입력으로 전환한다.
- attachment file/preview URL과 embed/response mapping을 UUID 기반으로 갱신한다.

추가 정리

- body 기반 UUID 입력(`target_uuid`, `parent_uuid`)도 path UUID와 동일하게 delivery 경계에서 형식 검증한다.
- `parent_uuid` 응답 projection은 게시글의 전체 댓글 재조회 대신, 현재 응답에 필요한 parent ID 집합만 조회하는 방식으로 축소한다.

관련 문서/코드

- `internal/domain/entity`
- `internal/application/port`
- `internal/application/service`
- `internal/delivery/http.go`
- `internal/delivery/response`
- `docs/API.md`
- `docs/ARCHITECTURE.md`

## 2026-03-15 - Hidden board 신고 경계와 댓글 parent 활성 규칙, suspension UUID 검증을 공개 계약에 맞춘다

상태

- decided

배경

- Round 27 리뷰에서 hidden board 비노출 정책과 댓글 reply 규칙, 일부 admin UUID path validation이 문서 계약과 다르게 동작하는 지점이 확인됐다.

관찰

- `ReportService.CreateReport`는 신고 대상의 존재 여부만 확인하고 hidden board visibility 정책을 재사용하지 않는다.
- 댓글 reply 생성은 같은 게시글/1-depth만 확인하고 삭제된 부모 댓글(tombstone) 여부는 거르지 않는다.
- suspension admin API는 `userUUID` path param을 UUID 형식으로 검증하지 않는다.

결론

- hidden board의 post/comment는 신고 경로에서도 non-admin에게 not found로 수렴시킨다.
- reply parent는 같은 게시글의 활성 댓글만 허용한다.
- suspension GET/PUT/DELETE도 다른 UUID path endpoint와 동일하게 delivery 경계에서 형식 검증 후 400을 반환한다.

후속 작업

- report service 테스트에 hidden board 대상 신고 차단 케이스 추가
- comment service 테스트에 deleted parent reply 거절 케이스 추가
- delivery 테스트에 malformed suspension UUID 거절 케이스 추가

관련 문서/코드

- `internal/application/service/reportService.go`
- `internal/application/service/commentService.go`
- `internal/delivery/http.go`
- `docs/API.md`

## 2026-03-15 - code-quality-auditor 스킬은 트리거·감사 절차·산출물 갱신 규칙을 명시한다

상태

- decided

배경

- 현재 `code-quality-auditor` 스킬은 목적과 출력 파일 위치만 간단히 적혀 있어, 다른 Codex가 언제 이 스킬을 써야 하는지와 어떤 순서로 검토/기록해야 하는지가 충분히 드러나지 않는다.
- `assets/TEMPLATE.md`는 `scaffold_doc.py`가 치환하지 않는 placeholder를 포함해 실제 재사용성이 낮다.
- `agents/openai.yaml`이 없어 UI 메타데이터와 기본 호출 문구도 제공되지 않는다.

결론

- 스킬 frontmatter description에 트리거 문장을 보강해 "코드 리뷰/리스크 점검/리뷰 문서 갱신" 요청에서 더 안정적으로 선택되게 한다.
- SKILL 본문에는 감사 전 준비, 기존 리뷰 이력 확인, AI review 작성 규칙, refactor backlog 분리 기준, no-finding 처리, write guardrail을 명시한다.
- 템플릿/참조 문서는 현재 `.documents/review` 구조와 일치하도록 정리하고, 스캐폴드 스크립트로 실제 사용할 수 있는 형태만 남긴다.
- `agents/openai.yaml`을 추가해 display name, short description, default prompt를 제공한다.

후속 작업

- 스킬 문서와 리소스를 갱신한다.
- 가능한 범위에서 스크립트/메타데이터 정합성을 검증한다.

관련 문서/코드

- `.agents/skills/code-quality-auditor/SKILL.md`
- `.agents/skills/code-quality-auditor/references/*`
- `.agents/skills/code-quality-auditor/assets/*`
- `.agents/skills/code-quality-auditor/agents/openai.yaml`

## 2026-03-15 - 댓글 projection과 hidden visibility guard는 service 내부 공통 helper로 수렴한다

상태

- decided

배경

- Round 28 리뷰에서 `parent_uuid` 조립과 hidden board visibility 확인은 동작은 맞지만 service별로 중복된다고 확인됐다.
- 현재 단계는 외부 계약 변경보다 DB 어댑터 전환 전에 read-side/helper 책임을 정리하는 편이 더 적절하다.

관찰

- `comment list`, `post detail`, 단일 comment 모델화가 모두 `parent_id -> parent_uuid` 해석 규칙을 공유한다.
- `CommentService`, `ReactionService`, `AttachmentService`, `ReportService`는 각자 board/post 조회 후 `EnsureBoardVisible`을 다시 조합한다.

결론

- `parent_uuid` projection은 `internal/application/service` 내부 전용 helper로 옮겨 comment 관련 read path가 재사용한다.
- hidden visibility 확인도 service 내부 helper로 묶어 board/post/comment target 확인 흐름을 공통화한다.
- 이번 정리는 포트/HTTP 계약을 바꾸지 않고 service 내부 중복 제거와 추후 저장소 최적화 진입점 정리에 집중한다.

후속 작업

- helper 단위 테스트를 추가해 visibility/not-found 수렴 규칙을 고정
- comment list/post detail/comment 단건 모델화가 같은 parent UUID helper를 사용하도록 정리
- comment/reaction/attachment/report service의 visibility lookup 중복을 helper로 대체

관련 문서/코드

- `internal/application/service/post_detail_query.go`
- `internal/application/service/commentService.go`
- `internal/application/service/reactionService.go`
- `internal/application/service/attachmentService.go`
- `internal/application/service/reportService.go`

## 2026-03-15 - Admin HTTP 권한 확인은 application port로 올리고 delivery의 repository 직접 의존을 제거한다

상태

- decided

배경

- Round 29 리뷰에서 admin HTTP 미들웨어는 보안상 유용하지만, 현재 구현이 repository port를 delivery 레이어까지 끌어올려 아키텍처 문서의 "delivery는 use case만 호출" 원칙과 어긋난다고 확인됐다.

관찰

- `adminRoleMiddleware`는 `UserRepository.SelectUserByID`를 직접 호출해 admin 여부를 판별한다.
- 실제 권한 판정 규칙은 이미 application 레이어의 `AuthorizationPolicy`와 `UserService`가 알고 있다.
- `PostService.commentFromEntity`는 현재 사용처가 없는 dead code다.

결론

- admin HTTP 미들웨어는 repository 대신 application port(`AdminAuthorizer`)만 의존한다.
- `UserService`가 `AdminAuthorizer`를 구현해 delivery 경계의 admin role 확인을 담당한다.
- 미사용 `PostService.commentFromEntity`는 제거해 comment projection 경로를 공통 helper로 단일화한다.

후속 작업

- middleware/HTTP 테스트를 `AdminAuthorizer` 기준으로 정리한다.
- composition root와 test wiring에서 `AdminAuthorizer`를 주입한다.
- 아키텍처 문서에서 admin HTTP 방어도 application port 경계로 설명한다.

관련 문서/코드

- `internal/application/port`
- `internal/application/service/userService.go`
- `internal/delivery/middleware/adminRoleMiddleware.go`
- `internal/delivery/http.go`
- `internal/application/service/postService.go`

## 2026-03-15 - Markdown 본문은 raw로 저장하고 HTTP rate limit은 read/write를 모두 다룬다

상태

- decided

배경

- Step 3의 입력 sanitizer는 저장 전에 `html.EscapeString`을 적용해 markdown 원문을 변형한다.
- 현재 공개 계약은 게시글 본문과 첨부 참조를 markdown(`attachment://{attachmentUUID}`)로 다루며, 코드 블록이나 raw HTML 예제도 원문 그대로 보존될 필요가 있다.
- 기존 HTTP rate limit은 쓰기 메서드만 제한해 GET flood에 대한 application-layer 방어가 없다.

관찰

- 입력 경계 escape는 XSS payload 저장을 줄이는 대신, markdown source of truth를 변경해 렌더링 책임을 backend 저장 단계와 섞는다.
- 이 서비스는 HTML을 서버에서 렌더링하지 않고 JSON API를 제공하므로, 원문 보존과 출력 렌더러의 escaping/sanitizing 책임 분리가 더 일관된다.
- 목록/상세 GET은 캐시가 있어도 반복 호출 시 애플리케이션과 저장소 자원을 계속 소비한다.

결론

- 게시판/게시글/댓글/신고/운영 메모 같은 텍스트 입력은 저장 전에 변형하지 않고 raw 그대로 use case로 전달한다.
- `delivery.http.sanitizer.enabled`와 `InputSanitizer` 기반 입력 escape 파이프라인은 제거한다.
- HTTP rate limit은 `/api/v1` 하위 IP 기준으로 read/write 모두 적용하되, 읽기 메서드(`GET/HEAD/OPTIONS`)는 별도 한도(`readRequests`)를 둔다.

후속 작업

- HTTP 테스트를 markdown raw 보존과 GET read rate limit 기준으로 갱신한다.
- 설정/문서에서 sanitizer 항목을 제거하고 `delivery.http.rateLimit.readRequests`를 추가한다.
- 프론트엔드 또는 향후 renderer 경계에서 output escaping/sanitizing 책임을 명시한다.

관련 문서/코드

- `internal/delivery/http.go`
- `internal/config/config.go`
- `internal/delivery/http_test.go`
- `docs/API.md`
- `docs/CONFIG.md`

## 2026-03-18 - Rate limit trust boundary, outbox lease, upload authz 순서, UUID 응답 계약을 함께 정리한다

상태

- decided

배경

- Round 30 리뷰에서 reverse proxy 경계, outbox 재전달, 업로드 자원 사용 순서, admin report 응답 식별자 정책, cache read stale window가 실제 리스크로 확인됐다.
- 이 이슈들은 각각 레이어가 다르지만, 모두 "외부 경계 계약을 먼저 고정하고 내부 구현이 그 계약을 지키도록 정리한다"는 같은 성격의 수정이다.

관찰

- 현재 HTTP rate limit은 `ClientIP()` 결과를 key로 쓰지만 trusted proxy 설정을 코드에서 명시하지 않는다.
- in-memory outbox는 processing lease를 30초 고정 timeout으로 reclaim해 장시간 handler에서 중복 dispatch 가능성이 있다.
- attachment upload는 권한 확인 전에 본문 전체를 읽고 MIME 검증/이미지 최적화를 수행한다.
- admin report 응답은 여전히 내부 숫자 FK(`target_id`, `reporter_user_id`, `resolved_by`)를 노출한다.
- comment/post 목록 cache loader는 바깥에서 읽은 parent snapshot을 재사용해, 삭제/visibility 변경과 경합하면 stale payload를 만들 수 있다.

결론

- HTTP 서버는 기본값으로 forwarded header를 신뢰하지 않도록 trusted proxies를 명시적으로 비활성화한다. 추후 reverse proxy 신뢰가 필요하면 설정 기반으로 열되, 기본은 보수적으로 둔다.
- outbox processing lease는 relay가 처리 중 heartbeat로 갱신하고, 저장소 reclaim은 "최근 heartbeat가 끊긴 processing만 stale"로 판단한다.
- attachment upload는 post/user/visibility/ownership 검사를 먼저 수행한 뒤에만 파일 본문을 읽고 최적화한다.
- admin report 응답은 운영 리소스 자체의 `report.id`는 유지하되, 연관 리소스와 사용자 참조는 UUID 중심(`target_uuid`, `reporter_uuid`, `resolved_by_uuid`)으로 정리한다.
- cache miss loader 내부에서 parent existence/visibility를 다시 확인해 stale non-empty 목록을 만들지 않게 한다.

후속 작업

- HTTP 테스트에 spoofed `X-Forwarded-For` rate-limit 우회 방지 케이스 추가
- outbox multi-worker + long-running handler 중복 dispatch 방지 테스트 추가
- attachment service 테스트에 unauthorized upload가 body를 읽지 않는 케이스 추가
- admin report response/swagger/API 문서를 UUID 응답 기준으로 정렬
- post/comment list read path가 delete/hide race에 stale payload를 만들지 않는 서비스 테스트 추가

관련 문서/코드

- `cmd/main.go`
- `internal/delivery/http.go`
- `internal/infrastructure/event/outbox/relay.go`
- `internal/infrastructure/persistence/inmemory/outboxRepository.go`
- `internal/application/service/attachmentService.go`
- `internal/application/service/commentService.go`
- `internal/application/service/postService.go`
- `internal/application/model/report.go`
- `internal/delivery/http_responses.go`
- `docs/API.md`
- `docs/ARCHITECTURE.md`
- `docs/CONFIG.md`

## 2026-03-18 - service layer 미사용 helper를 제거해 읽기 비용을 줄인다

상태

- decided

배경

- Round 30 후속 정리에서 service package 안에 실제 호출 경로가 없는 helper가 남아 있었다.
- 이들은 동작 버그는 아니지만, 탐색 시 "현재도 쓰이는 표준 경로인가?"라는 불필요한 판단 비용을 만든다.

관찰

- `appendEventsToOutbox`는 `dispatchDomainActions`의 단순 wrapper인데 호출처가 없다.
- `userUUIDByID`는 `userUUIDsByIDs` 단건 wrapper인데 호출처가 없다.

결론

- public-like helper surface는 실제 사용되는 경로만 남기고, 미사용 wrapper는 제거한다.
- 동작 변경이 없는 cleanup이므로 회귀는 기존 전체 테스트와 vet로 확인한다.

후속 작업

- dead code 정리 시 review backlog와 구현이 다시 어긋나지 않도록 라운드 종료 시점에 함께 청소한다.

관련 문서/코드

- `internal/application/service/outbox_events.go`
- `internal/application/service/user_reference.go`

## 2026-03-19 - soft-delete read-model, attachment revoke cache, post delete event 정합성을 맞춘다

상태

- decided

배경

- Round 31 리뷰에서 admin report 조회, attachment 삭제 직후 파일 접근, post 삭제 후 cache invalidation이 각각 다른 경계에서 어긋난 것이 확인됐다.
- 세 이슈 모두 "삭제/비노출 상태 전이 후 읽기 경계가 이전 상태를 얼마나 오래 보존할 수 있는가"에 대한 계약 불일치다.

관찰

- admin report projection은 post target UUID를 조회할 때 deleted post를 제외한다.
- attachment file 응답은 public cache를 허용하지만 attachment delete는 즉시 비노출을 의도한다.
- post delete 이벤트는 댓글 cascade를 수행하면서도 deleted comment ID 목록을 비워 보내 cache invalidation 의미를 충분히 전달하지 못한다.

결론

- report read-model은 soft-deleted target에도 안정적으로 UUID를 복원할 수 있어야 한다.
- attachment revoke 의미가 "즉시 접근 차단"이라면 file 응답 cache 정책도 그 의미에 맞춰 보수적으로 조정한다.
- post delete 이벤트는 하위 comment cascade 결과를 함께 전달하거나, 동일 수준의 cache invalidation 효과를 별도로 보장해야 한다.

후속 작업

- deleted post 대상 report 조회 회귀 테스트 추가
- attachment file cache header 회귀 테스트 갱신
- post delete가 comment reaction cache까지 무효화하는 테스트 추가

관련 문서/코드

- `internal/application/service/reportService.go`
- `internal/infrastructure/persistence/inmemory/postRepository.go`
- `internal/application/service/postService.go`
- `internal/application/event/cache_invalidation_handler.go`
- `internal/delivery/http.go`
- `docs/API.md`

## 2026-03-19 - in-memory hot path와 context 규칙 문서를 현재 설계에 맞춘다

상태

- decided

배경

- Round 32 리뷰에서 새 기능 버그보다는 in-memory hot path의 선형 스캔 비용과 문서-구현 경계 규칙 불일치가 남아 있었다.
- 특히 `UnitOfWork` snapshot 비용은 in-memory 한계로 당장 제외하더라도, rate limiter와 cache invalidation은 요청/이벤트 hot path라 개선 우선순위가 높다.

관찰

- in-memory rate limiter는 `Allow` 호출마다 전체 bucket map을 sweep한다.
- in-memory cache의 `DeleteByPrefix`는 전체 store scan 기반이다.
- 아키텍처 문서는 모든 경계 메서드에 `context.Context`가 온다고 설명하지만, pure value/crypto/helper 성격의 port는 예외다.
- mapper package에는 실제 사용처가 없는 exported helper가 남아 있다.

결론

- rate limiter cleanup은 요청당 full scan 대신 주기적 cleanup으로 바꾼다.
- in-memory cache는 key prefix index를 유지해 `DeleteByPrefix`가 전체 store scan 없이 동작하도록 바꾼다.
- context 전달 규칙 문서는 "I/O 또는 취소/추적이 필요한 경계" 기준으로 좁혀 실제 port 집합과 맞춘다.
- 미사용 exported mapper helper는 제거해 public surface를 실제 사용 계약에 맞춘다.

후속 작업

- rate limiter periodic cleanup 회귀 테스트 추가
- in-memory cache prefix index 정합성 테스트 추가
- 아키텍처 문서의 context 규칙 설명 갱신

관련 문서/코드

- `internal/infrastructure/ratelimit/inmemory/in_memory_rate_limiter.go`
- `internal/infrastructure/cache/inmemory/in_memory_cache.go`
- `internal/application/mapper/dto_mapper.go`
- `docs/ARCHITECTURE.md`

## 2026-03-19 - delivery 입력 enum을 application 모델로 올리고 cursor list orchestration을 공통 helper로 수렴한다

상태

- decided

배경

- Round 32의 남은 구조 항목은 delivery가 domain enum/parser를 직접 사용한다는 점과, board/post/comment 목록 read path가 동일한 cursor pagination orchestration을 반복한다는 점이었다.
- 이 두 항목은 기능 오류보다 경계 명확성과 유지보수 비용 문제에 가깝지만, 현재 설계 철학과 직접 맞닿아 있어 정리 가치가 높다.

관찰

- delivery request parser는 `reaction/report/suspension` 입력을 `internal/domain/entity` parser에 직접 의존한다.
- `ReactionUseCase`, `ReportUseCase`, `UserUseCase.SuspendUser` 포트도 domain enum 타입을 외부 계약으로 노출한다.
- `BoardService.GetBoards`, `PostService.GetPostsList`, `CommentService.GetCommentsByPost`는 `fetch limit+1 -> hasMore -> nextCursor` 흐름을 거의 동일하게 반복한다.

결론

- 공개 입력 enum은 application 모델 타입으로 승격하고, delivery는 protocol string을 application 모델로만 파싱한다.
- service 내부에서만 application 모델 enum을 domain enum으로 변환해 domain dependency를 안쪽 레이어로 밀어 넣는다.
- cursor 기반 목록 조회는 공통 helper로 `limit+1`, `hasMore`, `nextCursor` 계산을 수렴시켜 board/post/comment read path의 중복을 줄인다.

후속 작업

- delivery fake/usecase 회귀 테스트를 application 모델 입력 타입 기준으로 갱신
- cursor helper 단위 테스트 추가
- 아키텍처 문서의 reaction/input parsing 규칙 갱신

관련 문서/코드

- `internal/application/model/input_types.go`
- `internal/application/port/reaction_usecase.go`
- `internal/application/port/report_usecase.go`
- `internal/application/port/user_usecase.go`
- `internal/application/service/cursor_list.go`
- `internal/delivery/http_requests.go`
- `docs/ARCHITECTURE.md`

## 2026-03-19 - 공개 식별자 UUID 정책은 운영 리소스 예외만 남기고 문서 기준을 닫는다

상태

- decided

배경

- `docs/ROADMAP.md`의 Step 2에는 `공개 UUID 전환 이후 문서/운영 규칙 잔여 정리`가 아직 남아 있다.
- 공개 path/body 계약은 대부분 UUID로 전환됐지만, 문서에는 운영 리소스의 숫자 ID 예외와 일부 응답 필드 규칙이 충분히 정리돼 있지 않았다.
- 이 상태로 두면 `Report`, `Reaction`, `Tag` 같은 식별자 노출이 UUID 정책의 미완성인지 의도된 예외인지 다시 해석해야 한다.

관찰

- 공개 도메인 리소스(`User`, `Board`, `Post`, `Comment`, `Attachment`)는 path/body/response 참조가 이미 UUID 중심으로 정리돼 있다.
- 운영 리소스인 `reportID`, `messageID`는 추적성과 수동 운영 편의 때문에 숫자/opaque ID 유지가 실제 사용성에 유리하다.
- `Reaction.id`, `Tag.id`는 외부에서 후속 호출의 대상이 되는 식별자가 아니며, 공개 계약상 필수성이 낮다.
- admin 목록 API는 운영 추적 목적이라 `last_id` 기반을 유지하고, 공개 목록 API만 opaque `cursor`를 사용한다.

결론

- 공개 도메인 리소스 식별자는 UUID로 통일한다.
- 운영 리소스 식별자인 `reportID`, `messageID`는 UUID 정책의 예외로 유지한다.
- 운영 목록 API(`admin/reports`, `admin/outbox/dead`)는 `last_id` 기반 pagination을 유지하고, 공개 목록 API만 opaque `cursor`를 사용한다.
- 공개 응답에서 후속 참조에 쓰이지 않는 내부 숫자 식별자(`Reaction.id`, `Tag.id`)는 문서상 기본 식별자 정책의 예외로 취급하지 않는다.
- 문서는 "공개 도메인 리소스는 UUID, 운영 리소스만 추적용 ID 예외" 기준으로 정렬한다.
- 위 정리가 반영되면 ROADMAP의 `공개 UUID 전환 이후 문서/운영 규칙 잔여 정리` 항목은 완료로 본다.

후속 작업

- `docs/API.md`에 공개/운영 식별자 규칙과 pagination 예외를 명시
- `docs/ARCHITECTURE.md`에 식별자 정책 예외를 반영
- `docs/ROADMAP.md` 상태 메모를 완료 기준으로 갱신

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/ARCHITECTURE.md`
- `internal/delivery/http.go`
- `internal/delivery/response/types.go`

## 2026-03-19 - anonymous 대신 서버 발급 guest user를 first-class user로 도입한다

상태

- decided

배경

- 비로그인 사용자도 post/comment 쓰기 흐름을 사용할 수 있게 하려면, 현재의 `userID + OwnerOrAdmin` 중심 구조를 완전히 버리거나 우회할 방법이 필요했다.
- 현재 코드베이스는 `Post`, `Comment`, `Attachment`, `Reaction`, `Report` 전반이 실제 user row와 세션을 전제로 설계돼 있다.
- 완전 anonymous actor를 새로 도입하면 소유권 증명, 응답 projection, attachment preview 권한 모델까지 넓게 재설계해야 한다.

관찰

- 현재 쓰기 권한 경계는 bearer token -> `user_id` -> `SelectUserByID` -> `CanWrite`/`OwnerOrAdmin` 흐름으로 일관돼 있다.
- `author_uuid`, `user_uuid` 같은 외부 응답 계약도 실제 user row가 존재한다는 가정 위에 서 있다.
- 브라우저 최초 방문 시 guest token을 발급받는 흐름이면, 사용자는 가입 여부를 의식하지 않아도 되면서 backend는 기존 소유권 모델을 유지할 수 있다.

결론

- anonymous actor를 별도로 만들지 않고, 서버가 내부 규칙으로 생성하는 `guest user`를 first-class user로 도입한다.
- guest user도 일반 user와 동일하게 `id`, `uuid`, 세션을 가진다.
- guest 발급은 전용 API `POST /api/v1/auth/guest`로만 수행한다.
- guest 식별자(username/email/password)는 서버가 내부적으로 생성하며, 외부에는 노출하지 않는다.
- guest는 1차에서 post/comment create/update/delete까지만 허용한다.
- guest는 1차에서 draft/publish, attachment, reaction, report를 사용할 수 없다.
- guest는 일반 username/password login 대상이 아니다. 발급된 bearer token으로만 사용한다.
- guest -> 정식 회원 전환은 `POST /api/v1/auth/guest/upgrade`에서 수행하고, 성공 시 현재 guest token은 폐기한 뒤 새 bearer token을 재발급한다.
- upgrade는 기존 guest user row를 재사용하고, `uuid`, 작성물 소유권을 유지한다. 세션은 rotation한다.
- account delete는 guest에 열지 않는다. guest self-delete는 `403 Forbidden`으로 거절한다.
- guest lifecycle은 `pending`, `active`, `expired` 3상태로 둔다.
- guest 발급은 `pending guest 생성 -> token 발급 -> session 저장 -> active 전환` 순서로 처리한다.
- session 저장 실패 시 guest는 `expired`로 즉시 전환하고 에러를 반환한다.
- `pending`/`expired` guest는 인증과 write 권한에서 유효 사용자로 보지 않는다.
- durable session 저장소 전환 전에도 같은 상태 모델을 유지하고, session 저장소가 별도 시스템이더라도 동일한 보상 흐름으로 맞춘다.
- guest cleanup은 background job으로 처리한다.
- cleanup 대상은 오래된 `pending`/`expired` guest와, 세션 없음 + 작성물 없음 상태로 유예 시간이 지난 `active` guest다.
- 작성물이 있는 guest는 자동 삭제하지 않는다.
- guest cleanup 삭제 방식은 hard delete가 아니라 기존 user soft delete 정책을 재사용한다.

후속 작업

- user 모델에 guest lifecycle 상태/시점 필드 추가
- user/session service에 guest lifecycle 전이와 cleanup 유스케이스 추가
- user repository에 guest cleanup 후보 조회 규약 추가
- background job runner에 guest cleanup job 등록
- guest 허용/차단 서비스에서 `active guest`만 통과하도록 테스트와 구현 반영
- API/ROADMAP/ARCHITECTURE/CONFIG 문서 정합성 반영

관련 문서/코드

- `internal/domain/entity/user.go`
- `internal/application/service/userService.go`
- `internal/application/service/sessionService.go`
- `internal/application/service/postService.go`
- `internal/application/service/attachmentService.go`
- `internal/application/service/reactionService.go`
- `internal/application/service/reportService.go`
- `internal/delivery/http.go`

## 2026-03-21 - guest upgrade는 account orchestration에서 세션 교체와 함께 단일 성공 경계로 처리한다

상태

- decided

배경

- 현재 `POST /api/v1/auth/guest/upgrade`는 guest user row 승격과 bearer token 교체를 delivery에서 두 단계로 나눠 호출한다.
- 이 구조에서는 user row는 이미 정식 회원으로 바뀌었는데, 뒤이은 session delete/new session save가 실패해 `500`이 나는 부분 커밋이 가능하다.
- guest upgrade는 문서상 `기존 guest row 재사용 + 기존 token 폐기 + 새 token 발급`이 하나의 사용자 기대 동작이다.

관찰

- `UserService.UpgradeGuest`는 user row 변경만 책임지고, `SessionService.RotateToken`은 세션 교체만 책임진다.
- 둘 다 각각은 맞는 책임이지만, 현재처럼 delivery에서 순차 조합하면 중간 실패 보상이 없다.
- `AccountUseCase`는 이미 `delete me + session invalidation` orchestration 경계로 사용 중이라, 계정 전환과 인증 상태 전환을 함께 다루는 위치로도 적합하다.

결론

- guest upgrade 진입점은 delivery가 아니라 `AccountUseCase`로 올린다.
- `AccountUseCase`는 `guest 검증 -> 새 token 준비/저장 -> user row 승격 -> 기존 token 폐기`를 하나의 orchestration으로 수행한다.
- 세션 교체와 user 승격 사이 실패가 나면, 새 세션 삭제와 user row 복구를 포함한 보상 흐름으로 기존 상태를 유지한다.
- delivery는 guest upgrade에서 더 이상 `UserUseCase`와 `SessionUseCase`를 직접 조합하지 않고, account use case 한 번만 호출한다.
- 기존 `SessionService.RotateToken`은 다른 독립적 token rotation 용도로는 유지할 수 있지만, guest upgrade의 성공 경계를 보장하는 용도로는 직접 사용하지 않는다.

후속 작업

- `AccountUseCase`에 guest upgrade orchestration 메서드 추가
- guest upgrade rollback/compensation 테스트 추가
- HTTP handler를 새 account orchestration으로 전환
- API/ARCHITECTURE 문서에 guest upgrade 책임 경계 반영

관련 문서/코드

- `internal/application/port/account_usecase.go`
- `internal/application/service/account/service.go`
- `internal/application/service/user/service.go`
- `internal/application/service/session/service.go`
- `internal/delivery/http.go`

## 2026-03-21 - post search rebuild는 시작 시점 이후의 최신 인덱스 갱신을 덮어쓰지 않는다

상태

- decided

배경

- `PostSearchStore`는 `post.changed` 이벤트를 소비해 개별 문서를 비동기로 갱신하고, 필요 시 `RebuildAll`로 전체 인덱스를 다시 구성한다.
- 기존 `RebuildAll`은 문서 로딩을 락 밖에서 수행한 뒤 마지막에 `documents` 맵을 통째로 교체했다.
- 이 구조에서는 rebuild 시작 후 들어온 `UpsertPost`/`DeletePost`가 최신 상태를 반영하더라도, 뒤늦게 끝난 rebuild가 더 오래된 스냅샷으로 덮어쓸 수 있다.

관찰

- 부팅 시점에는 `RebuildAll` 뒤에 relay를 시작하므로 문제가 드러나지 않는다.
- 하지만 운영 중 수동 rebuild, 복구 rebuild, 향후 admin rebuild 같은 런타임 재구축 경로가 생기면 stale overwrite가 correctness 문제로 바뀐다.
- 검색 인덱스는 eventual consistency를 허용하지만, rebuild 자체가 이미 반영된 더 최신 개별 갱신을 되돌리면 안 된다.

결론

- `PostSearchStore`는 문서 단위 최신성 시각을 추적한다.
- `RebuildAll`은 로딩 시작 시각을 기준으로, 그 이후 `UpsertPost`/`DeletePost`가 갱신한 문서는 rebuild 결과로 덮어쓰지 않는다.
- `UpsertPost`와 `DeletePost`도 같은 최신성 기준을 사용해 더 오래된 갱신이 최신 문서를 되돌리지 않도록 한다.
- 이 보호 규칙은 in-memory reference adapter에도 적용하고, 이후 SQLite/외부 search adapter도 같은 의미론을 유지한다.

후속 작업

- `PostSearchStore`에 최신성 guard 추가
- concurrent rebuild/update 테스트 추가
- API/ARCHITECTURE 문서에 런타임 rebuild의 최신성 보장 메모 반영

관련 문서/코드

- `internal/infrastructure/persistence/inmemory/postSearchRepository.go`
- `internal/infrastructure/persistence/inmemory/postSearchRepository_test.go`
- `docs/API.md`
- `docs/ARCHITECTURE.md`

## 2026-03-21 - 다음 우선순위는 user lifecycle 강화로 두고 PointHistory와 notification external delivery는 현재 범위에서 제외한다

상태

- decided

배경

- 현재 `Notification`은 inbox 조회 경험까지 구현되어 있고, 사용자 요구도도 외부 push/mail/webhook 전달보다 화면 내 확인 기능에 더 가깝다.
- SQLite 전환은 outbox 하나만 durable하게 올리기보다, repository/search/outbox를 한 번에 외부 어댑터로 교체하는 편이 전체 구조와 운영 이해에 더 자연스럽다.
- `PointHistory`와 포인트 개념은 현재 제품 범위에서 필수 요구로 보이지 않는다.
- 반면 이메일 인증과 비밀번호 재설정은 실제 사용자 계정 lifecycle 완성에 직접 연결되는 남은 핵심 기능이다.

관찰

- 현재 로드맵에는 `PointHistory`, `durable outbox adapter(SQLite table)`, `Notification 외부 delivery(push/mail/webhook)`가 후속 항목으로 남아 있다.
- 하지만 현재 팀 판단은 다음과 같다.
  - `PointHistory`는 도입하지 않는다.
  - notification은 inbox UI/API까지만 우선 제공하고, 외부 delivery는 현재 범위에서 제외한다.
  - SQLite는 durable outbox 단독 도입이 아니라 외부 어댑터 일괄 전환 시점에 함께 도입한다.
- 이 판단을 반영하면 Phase 1의 다음 기능 우선순위는 `이메일 인증 / 비밀번호 재설정`이 된다.

결론

- `PointHistory`는 현재 로드맵 범위에서 제거한다.
- notification의 현재 목표는 inbox 확인 경험까지로 한정한다.
- push/mail/webhook 같은 notification external delivery는 현재 범위에서 제외하고, 필요가 생기면 별도 결정으로 다시 연다.
- SQLite 전환은 `repository + search + outbox`를 포함한 외부 어댑터 일괄 교체 단계에서 추진한다.
- durable outbox 단독 전환이나 CDC는 현재 우선순위에서 제외한다.
- 다음 구현 우선순위는 `이메일 인증`과 `비밀번호 재설정`으로 고정한다.

후속 작업

- `docs/ROADMAP.md`에서 PointHistory와 notification external delivery 항목 정리
- Step 5 설명을 SQLite 외부 어댑터 일괄 전환 기준으로 정리
- 다음 구현 범위를 email verification / password reset 설계로 전환

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/API.md`
- `docs/ARCHITECTURE.md`

## 2026-03-26 - code-quality-auditor 리뷰 저장소는 SQLite를 원본으로 하고 Markdown은 append export 로그로 유지한다

상태

- decided

배경

- `code-quality-auditor`는 현재 `.documents/review/AI_REVIEW.md`와 `.documents/review/REFACTOR_BACKLOG.md`를 직접 수정하는 방식으로 리뷰 결과를 누적한다.
- 이 방식은 사람이 읽고 Git diff를 보기에는 편하지만, round 번호, finding ID, backlog ID, open high 수, 상태 변경 이력을 구조적으로 검증하거나 조회하기 어렵다.
- 특히 `AI_REVIEW.md`는 round별 스냅샷이 한 파일에 append되고, `REFACTOR_BACKLOG.md`는 구조적 refactor 항목이 장기 누적되므로, 형식 규약과 중복 제어를 도구로 강제할 필요가 있다.

관찰

- 리뷰 결과는 고정된 구조를 가진다. round, finding, acceptance criteria, refactor backlog, backlog 상태 변화는 각각 분리된 엔티티로 다루는 편이 자연스럽다.
- 반면 기존 Markdown 파일은 팀이 직접 읽고 Git 이력으로 추적하는 용도로는 여전히 유용하다.
- 따라서 원본 저장소와 사람 친화적 로그 표현을 분리하는 편이 현재 목적에 맞다.

결론

- `code-quality-auditor`의 리뷰 원본 저장소는 `.documents/review/review.db` SQLite 파일로 고정한다.
- 기존 `.documents/review/AI_REVIEW.md`와 `.documents/review/REFACTOR_BACKLOG.md` 이력은 SQLite로 전량 마이그레이션한다.
- 향후 리뷰 실행은 SQLite에 먼저 기록한 뒤, 같은 실행에서 이번에 새로 생성된 review round와 backlog 변화만 Markdown에 append export 한다.
- `AI_REVIEW.md`는 round log로 유지하고, `REFACTOR_BACKLOG.md`는 backlog item log와 backlog update log를 함께 담는다.
- 현재 최신 상태 조회와 구조적 검증은 Markdown이 아니라 SQLite 질의를 기준으로 한다.

후속 작업

- `code-quality-auditor` 스킬에 SQLite 기반 CLI 추가
- legacy Markdown import와 append export 테스트 추가
- 스킬 문서와 참조 자료를 DB 기록 + append export 흐름으로 갱신

관련 문서/코드

- `.agents/skills/code-quality-auditor/SKILL.md`
- `.agents/skills/code-quality-auditor/references/formats.md`
- `.agents/skills/code-quality-auditor/references/CHECKLIST.md`
- `.documents/review/AI_REVIEW.md`
- `.documents/review/REFACTOR_BACKLOG.md`

## 2026-03-26 - guest upgrade는 replacement session이 durable 해진 뒤에 current session을 폐기한다

상태

- decided

배경

- guest upgrade는 여전히 user row 승격과 bearer token 교체를 하나의 성공 경계로 다루어야 한다.
- 다만 current session을 먼저 지우는 순서는 replacement session 저장 실패나 rollback 실패 시 기존 세션을 잃을 위험을 키운다.
- 기존 guest token을 가능한 오래 유지한 채 replacement session을 먼저 durable 하게 만들면, 실패 복구가 "기존 세션 유지" 쪽으로 단순해진다.

관찰

- account orchestration은 replacement token을 먼저 저장한 다음 user 승격과 current token 폐기를 이어서 수행할 수 있다.
- replacement token 저장이 실패하면 user row는 아직 손대지 않았으므로 current session을 그대로 유지할 수 있다.
- user 승격 성공 후 current token 폐기가 실패하면 replacement token만 되돌리고 기존 guest/session 상태를 유지할 수 있다.

결론

- `AccountService.UpgradeGuestAccount`는 replacement session 저장을 current session 폐기보다 앞에 둔다.
- replacement session 저장이 성공하기 전에는 current session을 폐기하지 않는다.
- replacement session 저장 또는 current session 폐기 중 실패가 나면, 기존 guest/session 상태를 유지하도록 보상 처리를 수행한다.

실행 결과

- `internal/application/service/account/service.go`의 guest upgrade 순서를 replacement-first로 정리 완료
- guest upgrade rollback regression test 추가 완료
- `docs/ARCHITECTURE.md`의 guest upgrade success boundary를 새 순서로 정합성 반영 완료

관련 문서/코드

- `docs/ARCHITECTURE.md`
- `internal/application/service/account/service.go`
- `internal/application/service/accountService_test.go`

## 2026-03-26 - Step 5 SQLite foundation uses an internal migration ledger first

상태

- decided

배경

- Step 5의 첫 실제 slice는 repository adapter 전환 전에 SQLite 파일 DB를 안정적으로 여는 기반이 필요하다.
- 아직 전체 repository/search/outbox 전환이 시작되지 않았으므로, 외부 migration 도구를 먼저 도입하면 의존성만 늘고 runtime behavior는 바뀌지 않는다.
- 첫 slice에는 ordered SQL 적용과 재실행 방지용 ledger가 핵심이다.

관찰

- 현재 runtime은 여전히 in-memory repository를 사용하지만, SQLite package만으로도 파일 DB open, pragma 적용, migration ledger 기록은 가능하다.
- `schema_migrations` 테이블은 첫 adapter slice에서 replay safety를 확보하기에 충분하다.

결론

- Step 5 foundation은 ordered `.sql` 파일과 내부 `schema_migrations` ledger로 시작한다.
- 다음 Step 5 slice는 User/Auth adapter wiring이다.
- 외부 migration tool(goose/golang-migrate)은 이후 필요성이 확인되면 다시 검토한다.

실행 결과

- `internal/infrastructure/persistence/sqlite/open.go`
- `internal/infrastructure/persistence/sqlite/migrate.go`
- `internal/infrastructure/persistence/sqlite/open_test.go`

관련 문서/코드

- `docs/ROADMAP.md`
- `internal/infrastructure/persistence/sqlite/open.go`
- `internal/infrastructure/persistence/sqlite/migrate.go`

## 2026-03-26 - Step 5 User/Auth slice is wired to the SQLite auth DB

상태

- decided

배경

- Step 5의 첫 실제 adapter wiring은 사용자 계정과 인증 토큰 계열을 durable storage로 옮기는 slice였다.
- user/account signup, guest upgrade, email verification, password reset은 모두 `UserRepository`와 token repository의 의미 계약에 직접 의존한다.
- 나머지 Board/Post 계열은 아직 in-memory UoW를 유지해도 auth slice의 의미를 검증할 수 있다.

관찰

- `cmd/main.go`는 이제 `cfg.Database.Path`를 사용해 SQLite auth DB를 열고, `UserRepository`/verification token repo/password reset token repo를 그 DB에 붙인다.
- `sqlite.UnitOfWork`는 auth repo를 tx-bound SQLite repository로 묶고, 나머지 repo는 기존 wiring을 유지한다.
- 전체 테스트가 통과해 기존 auth/service 계약을 깨지 않았음을 확인했다.

결론

- Step 5의 `User/Auth` slice는 완료된 것으로 본다.
- 다음 Step 5 slice는 `Board/Post` adapter wiring이다.
- 이후 순서는 roadmap 권장 순서대로 `Outbox -> Search`다.

실행 결과

- `cmd/main.go`의 auth wiring을 SQLite DB 기반으로 전환
- `internal/config/config.go`에 `database.path` 추가
- `internal/infrastructure/persistence/sqlite/*` auth repo/UoW 추가

## 2026-03-26 - Step 5 Board/Post repositories are wired into main and tx scope

상태

- decided

배경

- Board/Post slice는 auth slice보다 훨씬 넓어서, 먼저 SQLite repository와 검색 query adapter를 올려 두고 `cmd/main.go` wiring을 별도 단계로 분리하는 편이 안전하다.
- `postSearchRepository`는 in-memory projection 대신 SQLite tables를 직접 조회하는 방식으로 구현해도, 서비스 계약과 테스트를 유지할 수 있다.

관찰

- `internal/infrastructure/persistence/sqlite`에 `BoardRepository`, `TagRepository`, `PostTagRepository`, `PostRepository`, `PostSearchRepository`를 추가했다.
- repository contract tests와 search smoke test가 통과했다.
- `cmd/main.go`는 board/tag/post/post-tag/search를 SQLite DB에 붙이고, `sqlite.UnitOfWork`는 tx-bound SQLite repo를 사용한다.
- 전체 테스트가 통과해 `CreateBoard`, `CreatePost`, `DeletePost`, search path가 새로운 wiring에서 유지됨을 확인했다.

결론

- Step 5의 Board/Post slice는 wiring까지 완료다.
- 다음 Step 5 slice는 Outbox 내구화다.

실행 결과

- `internal/infrastructure/persistence/sqlite/board_repository.go`
- `internal/infrastructure/persistence/sqlite/tag_repository.go`
- `internal/infrastructure/persistence/sqlite/post_tag_repository.go`
- `internal/infrastructure/persistence/sqlite/post_repository.go`
- `internal/infrastructure/persistence/sqlite/post_search_repository.go`
- `internal/infrastructure/persistence/sqlite/repository_test.go`
- `cmd/main.go`
- `internal/infrastructure/persistence/sqlite/unit_of_work.go`

관련 문서/코드

- `docs/ROADMAP.md`
- `internal/infrastructure/persistence/sqlite/*`

관련 문서/코드

- `cmd/main.go`
- `internal/config/config.go`
- `internal/infrastructure/persistence/sqlite/user_repository.go`
- `internal/infrastructure/persistence/sqlite/token_repositories.go`

## 2026-03-26 - Step 5 Outbox is durable on SQLite and wired into relay/admin/tx append

상태

- decided

배경

- Outbox는 relay durability와 admin dead-message 운영 경로를 동시에 책임진다.
- in-memory outbox는 프로세스 재시작 시 메시지 상태를 잃기 때문에, Step 5의 마지막 infrastructure slice로 durable store가 필요했다.

관찰

- `internal/infrastructure/persistence/sqlite`에 `OutboxRepository`와 tx-bound appender를 추가했다.
- `cmd/main.go`는 SQLite outbox store를 relay와 admin usecase에 연결하고, `sqlite.UnitOfWork`는 tx append를 같은 DB로 보낸다.
- `SelectDead`, `MarkRetry`, `MarkDead`, `FetchReady`, `MarkSucceeded` contract tests와 전체 테스트가 통과했다.

결론

- Step 5의 Outbox slice는 완료다.
- 다음 Step 5 slice는 Search의 FTS5 전환이다.

실행 결과

- `internal/infrastructure/persistence/sqlite/outbox_repository.go`
- `internal/infrastructure/persistence/sqlite/outbox_repository_test.go`
- `internal/infrastructure/persistence/sqlite/migrations/0003_outbox.sql`
- `cmd/main.go`
- `internal/infrastructure/persistence/sqlite/unit_of_work.go`

관련 문서/코드

- `docs/ROADMAP.md`
- `internal/infrastructure/persistence/sqlite/*`

## 2026-03-26 - Step 5 Search is backed by SQLite FTS5 while ranking stays in Go

상태

- decided

배경

- Step 5의 마지막 search slice는 기존 ranking/cursor 계약을 깨지 않으면서 SQLite 기반 전문 검색으로 넘어가야 했다.
- 검색 순위는 이미 Go scoring helper로 정해져 있으므로, FTS5는 후보 집합 필터 역할만 맡기는 편이 의미 보존에 유리하다.
- FTS5 자체 ranking을 외부에 노출하면 현재 search cursor/score 의미와 어긋날 수 있다.

관찰

- 현재 search repository는 published post와 active tag를 조합한 문서를 만들고, 그 문서 전체를 기준으로 기존 BM25 유사 scoring을 계산한다.
- outbox relay가 search indexer를 호출하므로, search index는 개별 post 변경에 반응해 유지할 수 있다.
- FTS5는 rowid 기반 후보 필터로 쓰고, 실제 점수 계산과 cursor 비교는 기존 Go 로직을 유지할 수 있다.

결론

- SQLite search adapter는 `post_search_fts` virtual table을 유지한다.
- `RebuildAll`/`UpsertPost`/`DeletePost`는 이 FTS5 인덱스를 갱신하는 책임을 가진다.
- `SearchPublishedPosts`는 FTS5로 후보 row를 먼저 좁힌 뒤, 기존 Go scoring helper로 최종 순위를 계산한다.
- 외부 응답의 score/cursor semantics는 그대로 유지한다.

후속 작업

- `internal/infrastructure/persistence/sqlite/migrations/0004_search_fts.sql` 추가
- `internal/infrastructure/persistence/sqlite/post_search_repository.go`를 FTS5 기반 후보 필터로 전환
- search repository contract/maintenance 테스트 추가
- ROADMAP current state memo에 Search FTS5 완료 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `internal/infrastructure/persistence/sqlite/post_search_repository.go`
- `internal/application/port/post_search_repository.go`

## 2026-03-28 - SQLite search rebuild uses shadow replay with updated_at freshness

상태

- decided

배경

- SQLite search rebuild이 live FTS 테이블을 직접 덮으면, rebuild 중 발생한 최신 update/delete를 잃을 수 있다.
- 별도 version 컬럼 없이도 `posts.updated_at`은 projection freshness 기준으로 재사용할 수 있다.

결론

- `RebuildAll`은 shadow projection을 먼저 만들고, `posts.updated_at` 기준 delta를 replay한 뒤 live FTS와 교체한다.
- `UpsertPost`와 `DeletePost`는 rebuild의 replay/swap 구간과 충돌하지 않도록 repository 내부 writer gate를 공유한다.

후속 작업

- search rebuild race regression test를 유지한다.
- 다중 프로세스 구성이 필요해지면 DB-backed lock 또는 version cursor로 다시 올린다.

## 2026-03-26 - Step 5 remaining SQLite slices are comment -> reaction -> attachment -> report -> notification

상태

- decided

배경

- `docs/ROADMAP.md`의 Step 5는 auth, board/post, outbox, search가 닫힌 뒤에도 일부 도메인이 in-memory로 남아 있었다.
- 남은 도메인은 서로 의존성이 달라서, 댓글을 먼저 옮긴 뒤 reaction/attachment/report/notification 순으로 진행하는 편이 안전하다.
- ranking projection은 outbox 기반 read-side 성격이 강하므로, 이번 remaining slice 묶음에서는 별도 전환 대상으로 두지 않는다.

관찰

- `comment`는 `post deletion`, `guest cleanup`, `reaction/report`의 visibility check에 쓰인다.
- `reaction`은 `post/comment` target에 대한 unique upsert/delete semantics가 중요하다.
- `attachment`는 cleanup candidate 조회와 orphan/pending-delete lifecycle이 핵심이다.
- `report`는 duplicate reporter/target 방지와 admin list/resolve 흐름이 핵심이다.
- `notification`은 dedup, unread count, mark-read/read-all이 핵심이다.

결론

- 남은 SQLite 전환 순서는 `comment -> reaction -> attachment -> report -> notification`으로 고정한다.
- 각 slice는 migration, repository, contract/regression test, `cmd/main.go` wiring, `sqlite.UnitOfWork` tx binding을 포함한다.
- ranking projection은 이번 묶음에 포함하지 않는다.

후속 작업

- comment SQLite repository/migration/test 구현
- reaction SQLite repository/migration/test 구현
- attachment SQLite repository/migration/test 구현
- report SQLite repository/migration/test 구현
- notification SQLite repository/migration/test 구현
- wiring 및 문서 정합성 반영

관련 문서/코드

- `docs/ROADMAP.md`
- `cmd/main.go`
- `internal/infrastructure/persistence/inmemory/unitOfWork.go`

## 2026-03-26 - Step 5 remaining SQLite slices have been implemented and MQ bridge is excluded

상태

- decided

배경

- remaining persistence slices were implemented after the order was fixed to `comment -> reaction -> attachment -> report -> notification`.
- after the SQLite adapters were wired and tests passed, the only optional Step 5 item left was MQ bridge.

관찰

- `comment`, `reaction`, `attachment`, `report`, `notification` now have SQLite repositories and migrations.
- `sqlite.UnitOfWork` creates tx-bound repositories for those domains.
- `cmd/main.go` now wires the same SQLite DB into those repositories.
- Full test suite passes.

결론

- Step 5 is complete without introducing an MQ bridge.
- Step 6 remains the next implementation step.

후속 작업

- Step 6 observability/exceptions work
- keep Step 5 docs as completed, not as an open transition slice

관련 문서/코드

- `docs/ROADMAP.md`
- `cmd/main.go`
- `internal/infrastructure/persistence/sqlite/*`

## 2026-03-27 - Runtime logging uses stdout plus lumberjack rotation with structured panic recovery

상태

- decided

배경

- 운영 런타임 로그를 JSON 구조화 로그로 유지하면서, 장기 실행 시 stdout만으로는 보관/순환이 부족했다.
- HTTP, background job, outbox relay, process entrypoint에서 panic/recover 경계가 분산되어 있어, 동일한 구조화 로그 포맷으로 수렴시킬 필요가 있었다.

관찰

- `cmd/main.go`는 bootstrap logger 이후 `stdout + lumberjack` logger를 composition root에서 조립한다.
- `internal/delivery`는 gin recovery middleware를 구조화 로그 버전으로 교체했다.
- `internal/infrastructure/job/inprocess`와 `internal/infrastructure/event/outbox`는 panic을 recovery하고 stack trace를 포함해 기록한다.
- `internal/config`는 `logging.*` top-level 설정을 읽고 검증한다.

결론

- runtime logger는 `stdout`과 rotating file을 동시에 쓰는 JSON logger로 고정한다.
- panic/recover는 HTTP, job, relay, entrypoint에서 구조화 로그를 남기도록 통일한다.

후속 작업

- 운영 기본값이 과도하면 `maxSizeMB`/`maxBackups`를 후속 조정한다.
- 필요 시 metrics/alerting과 연결되는 log field 규약을 추가한다.

관련 문서/코드

- `cmd/main.go`
- `cmd/logger.go`
- `internal/delivery/recovery.go`
- `internal/infrastructure/job/inprocess/runner.go`
- `internal/infrastructure/event/outbox/relay.go`
- `internal/config/config.go`

## 2026-03-28 - SQLite bottleneck checks should use Go benchmarks plus db wait metrics

상태

- decided

배경

- `MaxOpenConns=1`은 정합성에는 안전하지만, 실제로 병목인지 여부는 측정 없이 단정하기 어렵다.
- SQLite read/write 경로마다 커넥션 대기와 실행 시간의 영향을 분리해서 봐야 한다.

결론

- SQLite 병목 여부는 Go benchmark로 확인한다.
- benchmark는 `MaxOpenConns`를 1, 2, 4처럼 바꿔가며 비교한다.
- `database/sql`의 `WaitCount`와 `WaitDuration` 증분을 함께 본다.

후속 작업

- search와 outbox처럼 DB를 많이 타는 path별 benchmark를 유지한다.
- 대기 지표가 거의 없고 p95도 변하지 않으면 `MaxOpenConns=1`은 병목이 아니다.
- 대기 지표가 늘고 latency도 같이 늘면 connection pool 또는 SQLite write serialization이 병목이다.

## 2026-03-28 - SQLite default MaxOpenConns is 2

상태

- decided

배경

- 1, 2, 4, 10 conn benchmark를 비교한 결과, 읽기 경로는 10에서 가장 빠르지만 쓰기 경로의 busy contention이 너무 커서 기본값으로는 과하다.
- 2는 read latency를 유의미하게 낮추면서도 write path의 busy/error 증가를 가장 억제하는 균형점이었다.

결론

- `internal/infrastructure/persistence/sqlite.Open`의 기본 `MaxOpenConns`는 2로 둔다.

후속 작업

- write-heavy workload가 늘어나면 `MaxOpenConns`를 재측정한다.
- read-only benchmark가 분리되면 read/write pool 분리 가능성을 다시 검토한다.

## 2026-03-28 - Step 2 domain expansion is complete; cross-domain soft delete remains separate

상태

- decided

배경

- ROADMAP의 Step 2에는 attachment, report, notification, tag, user lifecycle, ranking-adjacent read paths까지 포함돼 있었고, 현재 구현 기준으로 해당 범위는 모두 닫혔다.
- 다만 soft delete를 post/comment 등 다른 도메인으로 확장할지 여부는 제품 정책과 복구/보존 정책을 다시 봐야 하는 별도 판단이다.

관찰

- user account soft delete + 익명화는 이미 반영돼 있다.
- post/comment는 delete/tombstone 정책이 있지만, generic soft delete를 다른 도메인까지 일괄 확장하는 별도 규약은 없다.

결론

- ROADMAP의 Step 2는 완료 상태로 본다.
- soft delete의 다른 도메인 확장은 현재 범위에서 제외하고, 필요 시 별도 decision으로 다시 연다.

후속 작업

- ROADMAP current state memo를 완료 기준으로 정리한다.
- generic soft delete 확장 요구가 생기면 복구/보존 정책을 함께 다시 결정한다.

## 2026-03-28 - Step 8 adds an HTML-first Web UI/Admin Console and requires current-user plus draft-resume contracts

상태

- decided

배경

- 단일 바이너리 배포와 관리자 운영을 실제로 쓰려면 API만으로는 부족하고, 브라우저에서 바로 쓰는 UI shell과 admin console이 필요하다.
- 현재 API는 대부분의 public/read/write 흐름을 제공하지만, UI가 안정적으로 로그인 상태와 관리자 권한을 판별할 current-user summary와, 작성 중 초안을 다시 여는 draft recovery/resume 계약은 없다.

관찰

- `users/me` 계열은 notification/delete만 있고, 현재 사용자 상태를 반환하는 endpoint는 없다.
- post draft는 생성과 publish는 있지만, draft list/resume용 read contract는 없다.
- 브라우저 UI는 auth transport와 정적 자산/템플릿 서빙 방식을 먼저 정해야 한다.

결론

- ROADMAP에 Step 8 `Web UI / Admin Console`을 추가한다.
- Step 8은 HTML-first, Alpine.js 기반 UI shell로 둔다.
- UI를 위해 `GET /api/v1/users/me` 같은 current-user summary 계약과 draft list/detail 계약을 별도 작업으로 추가한다.
- draft recovery/resume 계약은 owner/admin 범위로 제한하고, publish-only detail과 분리한다.
- 이 작업들은 아직 구현되지 않았으므로 Step 8은 미착수로 유지한다.

후속 작업

- ROADMAP current state memo에 Step 8 미착수 반영
- current-user summary API와 draft recovery/resume API의 구체 route/response shape를 다음 단계에서 확정

## 2026-03-28 - Step 8 browser auth uses SameSite cookie-first transport with header compatibility for non-browser clients

상태

- decided

배경

- Step 8의 Alpine.js UI는 same-origin 브라우저에서 동작하므로, 토큰을 localStorage에 두고 JS 헤더로 반복 전송하는 방식은 XSS 리스크가 크다.
- 기존 JSON API는 `Authorization: Bearer <token>` header 계약을 이미 갖고 있어, 브라우저 UI와 비브라우저 API 클라이언트의 전송 수단을 분리하는 편이 자연스럽다.

관찰

- 로그인 응답은 현재 Authorization header를 반환한다.
- same-origin 배포와 HttpOnly cookie는 브라우저 UI에 더 안전한 transport다.
- public HTML 페이지는 직접 진입 가능하므로 SEO/소셜 미리보기용 `<head>` 메타 주입 규칙도 함께 필요하다.

결론

- Step 8 브라우저 auth transport는 `HttpOnly + Secure + SameSite=Lax` cookie 우선으로 둔다.
- 로그인/guest upgrade/logout 계열 응답은 브라우저 UI에서 `Set-Cookie`를 갱신/폐기하는 경계를 함께 가진다.
- JSON API의 Authorization header 계약은 비브라우저 클라이언트 호환용으로 유지한다.
- UI 직접 진입 라우트는 `html/template` 기반 SSR fallback으로 `Title`과 Open Graph 메타를 주입한다.
- draft recovery/resume는 `GET /api/v1/users/me/drafts`와 `GET /api/v1/posts/{postUUID}/draft`로 분리한다.

후속 작업

- ROADMAP Step 8 문구를 cookie-first transport와 SSR fallback 규칙으로 구체화
- auth transport와 SSR fallback 규칙이 실제 구현 순서를 바꾸지 않는지 확인

## 2026-03-29 - Step 8 guest bootstrap and login redirect are gated by current-user revalidation

상태

- decided

배경

- Step 8은 cookie-first transport와 `GET /api/v1/users/me` 기반 current-user store를 전제로 한다.
- 브라우저 최초 방문 시 guest 발급은 로그인 상태가 아닐 때만 실행돼야 하며, 보호 라우트 진입 후 로그인 복귀 경로도 고정해야 한다.

관찰

- current-user revalidation은 페이지 진입과 포커스 복귀 때 반복 실행될 수 있다.
- guest bootstrap을 revalidation 이전에 실행하면 이미 로그인된 사용자의 session을 불필요하게 guest로 덮어쓸 수 있다.
- protected route의 redirect target은 login success 시 복귀 규칙과 함께 정의돼야 한다.

결론

- 브라우저 최초 방문 시 guest 발급은 `GET /api/v1/users/me`가 `401`을 반환한 경우에만 실행한다.
- guest token은 `HttpOnly` cookie에만 저장하고, localStorage/sessionStorage에는 저장하지 않는다.
- 401 revalidation failure는 `/login?redirect={원래경로}`로 이동시키고, login success는 `redirect`가 있으면 그 경로로 복귀한다.
- `redirect`가 없거나 유효하지 않으면 login success의 기본 복귀 경로는 `/`로 둔다.

후속 작업

- ROADMAP Step 8의 guest bootstrap / login redirect 문구를 이 결정과 동일하게 유지
- UI 구현 시 current-user revalidation, guest bootstrap, login redirect의 순서를 regression test로 고정

## 2026-03-28 - Step 4 roadmap drops the generic hook system and keeps only the narrow action dispatcher boundary

상태

- decided

배경

- Step 4의 실제 구현은 outbox relay, mention 이벤트, notification, ranking read-side까지 완료됐고, 범용 hook system은 별도 요구 없이 남아 있었다.
- 코드베이스에는 `ActionHookDispatcher`가 outbox 기반 액션 전달 경계로 이미 자리잡았지만, `FilterHook` 같은 범용 hook placeholder는 실제 사용처가 없었다.

관찰

- `ActionHookDispatcher`는 서비스 계층에서 outbox 우선 전달과 fallback dispatch를 담당한다.
- `FilterHook`는 참조되지 않는 placeholder였다.
- ROADMAP의 hook system 항목은 현재 구현과 필요성에 비해 과했다.

결론

- ROADMAP에서 범용 hook system 항목을 제거한다.
- 코드에서는 `ActionHookDispatcher`만 좁은 액션 전달 경계로 유지하고, `FilterHook` placeholder는 제거한다.
- 향후 별도 플러그인/필터 확장이 실제 요구가 생기면 그때 다시 결정한다.

후속 작업

- `docs/ROADMAP.md`의 Step 4 완료 상태와 정합성 맞추기
- `internal/application/port/hook.go`의 미사용 placeholder 정리

## 2026-03-29 - error taxonomy expands to sqlite/mail/storage operational kinds

상태

- decided

배경

- public HTTP 응답은 이미 정규화되어 있지만, 운영 로그에서 DB busy/constraint, 메일 전송 실패, 파일 스토리지 실패가 전부 `internal server error`로 뭉개져 원인 추적성이 부족했다.
- Step 6의 목표는 public response 변경이 아니라 internal diagnosis 향상이므로, 공개 계약은 그대로 두고 내부 taxonomy만 세분화하는 것이 적절하다.

관찰

- `internal/customerror/customError.go`의 내부 분류는 `repository`, `cache`, `token` 3개뿐이었다.
- SQLite 런타임은 `modernc.org/sqlite`의 `Code()`를 통해 busy/locked, constraint, foreign key를 분리할 수 있다.
- mail delivery와 attachment storage 실패는 상위 계층에서 전부 `internal server error`로 수렴하고 있었다.
- SQLite repository 초기화 가드는 일부 위치에서 `ErrInternalServerError`를 직접 반환해 같은 저장소 계열 실패와 일관성이 없었다.

결론

- 내부 taxonomy에 `ErrSQLiteBusy`, `ErrSQLiteConstraint`, `ErrSQLiteForeignKey`, `ErrMailDeliveryFailure`, `ErrStorageFailure`를 추가한다.
- public HTTP 응답은 그대로 유지하고, 내부 로그와 `errors.Is` 기반 분기만 세분화한다.
- `WrapRepository`는 SQLite code를 감지하면 repository failure와 SQLite kind를 함께 보존한다.
- mail delivery와 file storage 실패는 각자의 전용 internal kind로 감싸고, SQLite repository 초기화 가드는 repository failure 체인으로 통일한다.

후속 작업

- ROADMAP Step 6의 internal taxonomy 표기를 최신화한다
- SQLite/helper/test와 mail/storage handler에서 새 internal kind가 보존되는지 회귀 테스트를 추가한다

관련 문서/코드

- `docs/ROADMAP.md`
- `docs/ARCHITECTURE.md`
- `internal/customerror/customError.go`
- `internal/application/event/mail_delivery_handler.go`
- `internal/application/service/attachment/handlers.go`
- `internal/infrastructure/persistence/sqlite/helpers.go`
