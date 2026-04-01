# go-comu-bin 로드맵

이 문서는 go-comu-bin의 개발 순서를 명확히 고정하기 위한 실행 로드맵입니다.
진행 중 예외 수정은 허용하지만, 기본 우선순위는 아래 순서를 따릅니다.

## 전체 방향

- Phase 1: 애플리케이션 코어 완성 (순수 Go, In-Memory 중심)
- Phase 2: 인프라스트럭처 어댑터 결합 (SQLite 중심 내구성 강화)
- Phase 3: 프로덕션 레벨 고도화 및 배포

## Phase 1. 애플리케이션 코어 완성

외부 인프라 연동을 최대한 미루고, 도메인/유스케이스/포트의 완성도를 먼저 끌어올립니다.

### Step 0. Config 관리 구조 도입 (최우선)

- 목표: 설정값 하드코딩 제거, 정적 설정 주입 기반 확보
- 예시 대상: 서버 포트, DB 경로, JWT 시크릿
- 구현 메모: `viper` 기반 로딩 + 검증 + 오타 키 실패 처리
- 인프라 선행 안전장치
  - DB 스키마 버전 관리 도입(`golang-migrate` 또는 `goose`)
  - 부팅 시 자동 마이그레이션 파이프라인(개발/배포 환경 공통) 정의
  - 이를 통해 SQLite 단일 파일(`data.db`)의 버전 업 시 스키마 불일치/데이터 손상 리스크를 최소화합니다.

### Step 1. 인증/인가(Auth) 미들웨어 적용

- 목표: 보호 라우트의 인증/인가 경계 확립
- JWT 토큰 검증
- Context를 통한 `user_id` 전달
- role 기반 권한 정책 적용

### Step 1.5 API 문서화 자동화 기반 구축

- 목표: Headless API 엔진의 계약(Contract)을 코드와 함께 유지
- Code-First 문서화: `swaggo/swag`, `gin-swagger` 기반 OpenAPI 스펙 자동 생성
- 서빙 엔드포인트: `/swagger/index.html`
- 구현 메모: API가 커지기 전에 주석/어노테이션 규칙부터 팀 표준으로 고정

### Step 2. 비즈니스 고도화 및 도메인 확장

- 신규 도메인
  - Attachment (파일 메타)
  - Report (신고)
  - Notification (알림)
  - Tag (다차원 분류, Post와 N:M)
- 운영/제재 규칙
  - 유저 정지(Suspension)
  - 게시물 임시저장(Draft)
  - Soft Delete
    - 현재 사용자 계정에는 soft delete + 익명화가 적용됨
    - 향후 게시글/댓글 등 다른 도메인으로 확장할지 결정 필요
- 유저 생명주기 관리
  - 이메일 인증(Email Verification) 파이프라인
  - 비밀번호 재설정(Password Reset) 토큰/만료/재사용 방지 정책
  - SMTP 어댑터 분리를 전제로 한 포트 설계
  - 유저 탈퇴 시 PII 삭제/비식별화 정책 고도화
    - 현재 방향: soft delete + 작성자 비식별화(익명화)
    - 삭제 대상: 이메일, 접속 IP, 전화번호, 외부 OAuth 식별자
    - 작성물 처리: 서비스 무결성을 위해 작성자 비식별화 유지
    - 장기 과제: soft delete 데이터의 보존기간, 복구권한, 추가 파기 절차 명확화
- 구조 개선
  - Offset -> Cursor 페이지네이션으로 DTO/Port 재설계
    - 완료: 공개 API의 `Board/Post/Comment/Attachment` 식별자는 UUID로 전환했고, 공개 목록 조회는 opaque `cursor`를 사용한다.
  - 공개 식별자 정책 고정
    - 완료: 공개 식별자는 `slug` 대신 `uuid`로 고정했다.
    - 비범위: SEO/slug URL 정책은 현재 채택하지 않는다.
  - Reaction 중복 방지 정책 도입
    - 동일 사용자의 리액션은 내부 저장 기준 `target_id`/`target_type` 단일 상태로 유지
    - `me` 기준 upsert/delete API를 통해 생성/변경/삭제를 target 중심으로 처리
    - SQLite 도입 시 `user_id`, `target_id`, `target_type` 복합 유니크 키로 데이터 무결성 보장

### Step 3. 어뷰징 방지 및 보안 로직 도입

- Rate Limit 포트/기본 구현 도입 (도배 방지)
- Sanitizer 파이프라인 구축 (XSS 방어)

### Step 4. 이벤트 기반 아키텍처 도입

- 트랜잭션 내부 outbox append + relay 비동기 소비 경로 전환
- 전달 보장 정책 고정: at-least-once (retry/backoff + dead 상태 보존)
- 멘션(Mention) 이벤트 유스케이스 추가
  - FE가 명시적으로 전달한 mention 대상 목록 -> `MentionedEvent` 발행
  - Notification 도메인과 연결해 재방문 유도 흐름 구축

## Phase 2. 인프라스트럭처 어댑터 결합 (SQLite All-in-One)

Phase 1에서 정의한 포트에 대해, 외부 의존 최소화 원칙으로 어댑터를 결합합니다.

### Step 5. 외부 인프라 어댑터 점진적 전환 (Phased Transition)

- 전환 원칙: Big Bang 방식의 일괄 교체를 지양하고, 도메인/기능 단위로 단계적 전환
  - 권장 순서: User/Auth -> Board/Post -> Outbox -> Search
  - 각 단계마다 계약 테스트/회귀 테스트/운영 관측 지표를 기준으로 안정화 후 다음 단계로 진행
- RDB Repository: SQLite 어댑터 (WAL 모드)
- Search: SQLite FTS5 기반 전문 검색
- Outbox 내구화: SQLite Outbox 테이블 + relay 전환
- Message Queue(선택): 외부 MQ 연계 브릿지 어댑터
- Object Storage: Local File System (파일은 디스크, 메타데이터는 DB)
- Cache: Ristretto 또는 고성능 인메모리 캐시 구현체
  - 캐시 무효화 전략 개선
    - 현재 서비스 계층 수동 무효화 방식에서 발생하는 불일치 리스크를 줄이기 위해, 이벤트 기반 무효화 또는 중앙 집중식 캐시 정책으로 전환
    - 캐시 쓰기/삭제 책임을 분산시키지 않도록 공통 계층 또는 이벤트 소비 지점으로 수렴
- 비범위
  - durable outbox만 단독으로 먼저 도입하는 단계는 두지 않는다.
  - CDC는 현재 채택하지 않는다.
  - Notification 외부 delivery(push/mail/webhook)는 현재 범위에서 제외한다.

## Phase 3. 프로덕션 고도화 및 배포

### Step 6. 관측성(Observability) 및 예외 처리 고도화

- `log/slog` + `lumberjack` 기반 JSON 로그 로테이션
- 글로벌 커스텀 에러 핸들러
- 에러 핸들링 전략 세분화 및 로깅 강화
  - `customerror.ErrInternalServerError`에 과도하게 수렴하는 흐름을 줄이고, DB/캐시/외부 연동 등 원인별 에러 타입을 구체화
  - `fmt.Errorf("...: %w", err)` 기반 에러 래핑으로 컨텍스트를 유지해 디버깅 가능성 확보
  - 글로벌 에러 핸들러 또는 서비스 레이어에서 구조화 로그를 남겨 운영 중 원인 추적성과 관측성 강화
- 데이터 내구성(Data Durability) 강화
  - SQLite 스냅샷 자동 백업(`.bak`) 스케줄러(Cron)
  - 오브젝트 스토리지(S3 등) 원격 백업 업로드
  - 후보: Litestream 기반 실시간 증분 백업 어댑터 검토

현재 반영 상태

- `customError`를 공개 에러와 내부 원인 분류(`repository`, `cache`, `token`)로 재구성
- 서비스 전반에 에러 래핑 적용
  - user, session, board, post, comment, reaction
- HTTP 에러 응답을 공개 에러 기준으로 정규화
  - 내부 래핑 메시지는 응답에 노출하지 않음
- `log/slog` 기반 구조화 에러 로그 도입
  - 요청 method/path/status/user_id(가능 시)와 내부 에러 체인을 함께 기록
- `log/slog + lumberjack` JSON 로그 로테이션을 런타임에 적용
  - stdout 유지 + 파일 로테이션 동시 기록
  - HTTP panic recovery, background job panic recovery, outbox relay panic logging 반영
- `AccountUseCase`를 통해 계정 삭제와 세션 무효화 orchestration 분리
- 사용자 탈퇴는 soft delete + 익명화 정책으로 전환
  - 기존 username/password는 재사용 불가 상태로 비식별화
  - 세션 정리는 best effort로 유지
- 사용자 공개 식별자로 `uuid` 도입
  - 내부 PK/FK는 `int64` 유지
  - post/comment/reaction 응답은 `author_uuid`, `user_uuid` 노출
- Reaction API를 `me` 기준 upsert/delete로 정리
  - `PUT /posts/{postUUID}/reactions/me`
  - `DELETE /posts/{postUUID}/reactions/me`
  - `PUT /comments/{commentUUID}/reactions/me`
  - `DELETE /comments/{commentUUID}/reactions/me`
- 저장소 contract test 확대
  - `ReactionRepository`
  - `UserRepository`
  - `BoardRepository`

남은 작업

- SQLite/외부 어댑터 도입 후 원인별 에러 타입 추가 세분화

### Step 7. 배포 환경 구축

- 리버스 프록시 + TLS 종료(nginx/caddy/LB 선택)
- 단일 실행 파일(`bin/commu-bin`) + `data.db` 배포
- 대상: M4 Mac mini 홈 서버
- 정적 자산/HTML 템플릿은 `go:embed`로 단일 바이너리에 포함 (Step 8 Web UI 배포 전제)

### Step 8. Web UI / Admin Console

- 목표: 단일 배포에서 바로 사용할 수 있는 HTML-first 웹 UI와 관리자 패널 제공
- 렌더링 전략: Alpine.js 기반의 server-rendered HTML + 최소 JS 조립
- 배포 전제
  - same-origin auth transport는 `HttpOnly + Secure + SameSite=Lax` cookie 우선으로 두고, JSON API의 `Authorization` header 계약은 비브라우저 클라이언트 호환용으로 유지
  - 정적 자산/템플릿 서빙: `go:embed`를 기본 전략으로 HTML 템플릿과 정적 파일을 단일 바이너리에 포함한다 (Step 7과 연계)
  - API 계약은 기존 JSON API를 재사용하되, UI 조립에 필요한 조회 계약을 보강
  - 직접 진입 라우트는 Go `html/template`로 `<head>` 메타데이터(Title, Open Graph)를 주입하는 SSR fallback 규칙을 둔다
  - Cookie/Bearer 병행 미들웨어: 인증 미들웨어는 `Cookie` 우선 + `Bearer` fallback 순서로 처리하고, 인증 실패 응답은 브라우저 요청(`Accept: text/html`)에는 로그인 페이지로 redirect, API 클라이언트에는 `401 JSON`으로 분기한다
  - CSRF 방어: `SameSite=Lax` cookie만으로 충분하지 않은 cross-origin form 공격에 대비해, state-mutating 엔드포인트 전체에 CSRF double-submit cookie 전략을 적용한다
- UI 페이지
  - Public shell: feed, board, tag, search
  - Post detail: 본문, 댓글, 대댓글, 리액션(현재 사용자 상태 포함), 첨부, 신고 진입
  - Post editor: draft 생성, 임시저장, 첨부, 발행, draft resume
  - Auth/account: signup, login, guest, guest upgrade, email verification(링크 진입 + confirm), password reset(링크 진입 + confirm)
  - Notifications: inbox, unread count, read-all, 단건 read
  - My page: current user state, logout, account delete
  - Admin console: reports, dead outbox, user suspension, board visibility
  - Error pages: 403, 404, 500
- 브라우저 URL 맵
  - `/api/v1`은 JSON API 전용이고, 아래 경로들은 브라우저 UI 전용이다
  - `/` → feed (전체 피드, SSR fallback)
  - `/boards/{boardUUID}` → board 게시글 목록
  - `/boards/{boardUUID}/posts/new` → 게시글 작성 (published)
  - `/boards/{boardUUID}/posts/drafts/new` → draft 생성
  - `/posts/{postUUID}` → post detail
  - `/posts/{postUUID}/edit` → post editor (draft resume / 수정)
  - `/tags/{tagName}` → tag 게시글 목록
  - `/search` → 검색 결과 (`?q=` 파라미터)
  - `/signup` → 회원가입
  - `/login` → 로그인
  - `/verify-email` → email verification confirm (`emailVerification.baseURL` 설정값과 일치)
  - `/reset-password` → password reset confirm (`passwordReset.baseURL` 설정값과 일치)
  - `/notifications` → notification inbox
  - `/me` → my page (현재 사용자 상태, 로그아웃, 계정 삭제)
  - `/admin/reports` → admin 신고 목록/처리
  - `/admin/outbox` → dead outbox 관리
  - `/admin/boards` → 게시판 visibility 관리
  - `/admin/users/{userUUID}/suspension` → 사용자 제재 (admin 전용)
  - 모든 직접 진입 라우트는 Go `html/template` SSR fallback으로 처리한다
  - 인증이 필요한 라우트에서 유효한 cookie가 없으면 `/login?redirect={원래경로}`로 redirect한다
  - admin 전용 라우트에서 권한이 없으면 `403` error page로 redirect한다
- Guest lifecycle UI 흐름
  - 브라우저 최초 방문 시에는 먼저 `GET /api/v1/users/me`로 현재 인증 상태를 확인하고, `401`인 경우에만 `POST /api/v1/auth/guest` 자동 호출로 guest token을 발급한다
  - guest token은 `HttpOnly` cookie에만 저장한다 (localStorage 사용 금지)
  - guest 상태에서 upgrade가 필요한 기능(reaction, attachment, report, draft/publish) 접근 시 업그레이드 인터스티셜(모달/배너) 표시 조건을 정의한다
  - guest 작성 콘텐츠의 소유권은 upgrade 성공 후에도 동일 `uuid`/`id`로 유지됨을 UI 흐름에서 반영한다
- 인증 토큰 저장소 및 동기화 규칙
  - auth 토큰(session cookie)의 단일 소스는 `HttpOnly` cookie이며, JavaScript로 직접 접근할 수 없다
  - localStorage/sessionStorage에는 auth 토큰을 저장하지 않는다 (XSS 탈취 방지)
  - Alpine.js store의 current user 상태(`uuid`, `username`, `role`, `email_verified`, `status`)는 `GET /api/v1/users/me` 응답을 단일 소스로 하며, 페이지 진입 시 fetch 후 store에 reactive하게 유지한다
  - 탭 간 동기화는 별도 메커니즘 없이 각 페이지 로드 및 포커스 복귀(`visibilitychange`) 시 `GET /api/v1/users/me` 재검증으로 처리한다
  - 로그인, 로그아웃, 계정 삭제 후에는 서버 cookie 삭제와 함께 Alpine.js store를 초기화한다
  - `GET /api/v1/users/me`가 `401`을 반환하면 store를 비우고 필요한 경우 `/login`으로 redirect한다
  - 로그인 성공 시 `redirect` 쿼리스트링이 있으면 해당 경로로 복귀하고, 없거나 유효하지 않으면 `/`로 보낸다
- CSRF 및 SSR 렌더링 보안 경계
  - CSRF token 주입: SSR 렌더링 시 Go `html/template`이 `<meta name="csrf-token" content="{{ .CSRFToken }}">` 형태로 CSRF token을 페이지에 주입한다
  - Alpine.js는 모든 state-mutating 요청(POST/PUT/PATCH/DELETE)에 `X-CSRF-Token` 헤더를 자동으로 첨부한다
  - 서버 delivery 미들웨어는 cookie 기반 요청의 state-mutating 엔드포인트에서 `X-CSRF-Token` 헤더를 검증하며, 불일치 시 `403 Forbidden`을 반환한다
  - CSRF 검증은 `Authorization: Bearer` 헤더를 사용하는 비브라우저 클라이언트(cookie 없는 요청)에는 적용하지 않는다
  - SSR 렌더링 보안 경계
    - Go `html/template`의 context-aware 자동 escaping을 기본으로 적용한다
    - 사용자 입력 데이터(post title, username 등)를 SSR template에 주입할 때 `template.HTML` 등 신뢰 타입으로 캐스팅하지 않는다
    - admin 도구를 포함한 모든 경로에서 DB에서 가져온 값은 raw HTML 삽입 경로를 허용하지 않는다
  - Content-Security-Policy
    - `script-src 'self'`를 기준으로 하고, Alpine.js 등 CDN 의존 시 SRI hash로 고정한다
    - `unsafe-inline` 스크립트는 허용하지 않는다
    - `object-src 'none'`, `base-uri 'self'`를 함께 설정한다
- UI/UX 개선 백로그 (Neo-Brutalism 유지 + 사용성 강화)
  - 레이아웃/공간 구조
    - 메인 콘텐츠 가독성: 가독성이 중요한 본문 영역은 .page-main 등으로 분리하여 max-width: 860px를 적용하고, 목록 및 관리자 페이지는 더 넓은 폭을 허용하도록 레이아웃 정책을 이원화한다
    - Empty state 행동 유도: `.empty`를 단순 텍스트 대신 중앙 정렬 카드(충분한 padding) + "글쓰기" CTA 버튼 구성으로 전환한다
  - 시각 정체성/인터랙션
    - 하드 섀도우 일관 적용: 루트 `--shadow` 토큰(`4px 4px 0 #1f2629`)을 `.button`, `.panel`, `.chip`, `.form input`, `.form textarea` 기본 스타일로 확장한다
    - 클릭 물리감(interaction state): 주요 인터랙티브 컴포넌트 :hover 시 미세 이동 피드백을 제공하고, :active는 --shadow-offset 토큰(예: 4px)을 기준으로 transform: translate(var(--shadow-offset), var(--shadow-offset)); box-shadow: none;를 적용한다
    - 폼 포커스 접근성 복구: 입력 컴포넌트의 `outline: none` 제거 또는 `:focus { outline: 2px solid var(--accent); outline-offset: 2px; }`를 공통 규칙으로 강제한다
  - 모바일 반응형 UX
    - 피드 정보 밀도 유지: `@media (max-width: 960px)`에서도 `.feed-item`은 `grid-template-columns: 3.5rem 1fr` 비율을 기본 유지하고, 필요 시 리액션 rail을 본문 하단의 수평 row로 재배치한다
    - 관리자 테이블 모바일 카드화: `.table-head/.table-row`의 단순 1열 축소 대신 `flex-direction: column` 카드 레이아웃으로 전환하고, 각 필드에 라벨(`::before`)을 부여해 column 의미를 보존한다. 이를 위해 템플릿(`page.tmpl` 내 admin table)에 `data-label` 속성을 선행 추가한다
    - 하단 네비게이션 터치 타겟 개선: `.bottom-nav-item`의 상하 padding을 확대하고, `.is-active` 상태는 아이콘 weight/배경 명도 대비를 강화해 현재 위치를 즉시 식별 가능하게 한다
  - 구현/검증 순서(UX 작업 공통)
    - 디자인 토큰 정리(그림자/포커스/spacing) → 컴포넌트 단위 반영(button/panel/form/nav) → 페이지 단위 반응형 튜닝(feed/admin) 순으로 진행한다
    - 변경마다 Playwright visual snapshot(Desktop/Mobile)과 키보드 포커스 이동(tab order, visible focus ring) 점검을 필수 체크로 둔다. visual 회귀 테스트 수행 전 `playwright.config.ts`의 visual 프로젝트에 모바일 디바이스 프로필(예: `Mobile Chrome`)을 포함한다

- 클라이언트 마크다운 XSS 방어
  - 서버는 raw markdown을 저장하고, 클라이언트 렌더링 시 DOMPurify + element/attribute allowlist로 sanitized HTML을 출력한다
  - 허용 목록 밖 태그/속성은 strip 처리하며, 외부 서비스로의 자동 요청을 유발하는 속성(예: `onerror`, `onload`)은 항상 제거한다
- 계약 보강
  - current user summary API 추가
    - 예: `GET /api/v1/users/me`
    - header/nav/admin gating을 위한 `uuid`, `username`, `role`, `email_verified`, `status` 조립 계약
  - draft recovery/resume API 추가
    - draft list: `GET /api/v1/users/me/drafts`
    - draft detail: `GET /api/v1/posts/{postUUID}/draft`
    - editor resume용 draft 단건 조회는 publish-only detail과 분리한다
    - draft list/detail 계약은 owner/admin 전용으로 둔다
  - login/logout cookie 계약 확정
    - `POST /api/v1/auth/login` 성공 시 `Set-Cookie: session=...; HttpOnly; Secure; SameSite=Lax` 반환 여부 및 브라우저 클라이언트 식별 조건 확정
    - `POST /api/v1/auth/logout` 성공 시 cookie 삭제(`Max-Age=0`) 처리 추가
    - Bearer 계약은 비브라우저 클라이언트 호환용으로 병행 유지
  - 현재 사용자 리액션 상태 조회 계약 추가
    - post/comment detail 응답에 `my_reaction_type` 필드 추가 (인증된 경우에만, 미인증/guest 시 `null`)
    - UI의 like/dislike 버튼 highlight 렌더링에 사용
  - email verification / password reset 진입 라우트 계약
    - `GET /{verifyEmailPath}?token=<token>` 진입 → 클라이언트에서 `POST /api/v1/auth/email-verification/confirm` 호출
    - `GET /{resetPasswordPath}?token=<token>` 진입 → 클라이언트에서 `POST /api/v1/auth/password-reset/confirm` 호출
    - 두 라우트는 SSR fallback으로 Go `html/template` 진입 페이지를 반환하고, 서버는 token 유효성을 미리 확인하지 않는다

## 운영 관점 우선순위 메모

- 위 확장 항목(이메일 인증/비밀번호 재설정, 태그, 멘션, 자동 백업)은 상용 서비스 안정성을 위해 중요도가 높음
- 단, 초기 개발 속도를 위해 Phase 1의 기존 핵심 경로를 먼저 완주한 뒤 Step 2와 4 내 세부 태스크로 병행 착수 가능

## 현재 상태 메모

- Step 0 완료 (Config)
  - `viper` 기반 로딩/검증, unknown key 실패(`UnmarshalExact`), env override 반영
  - `event.outbox.*`, storage provider, bootstrap admin, jobs 설정 반영
- Step 1 완료 (인증/인가)
  - JWT + 세션 검증 미들웨어
  - role 기반 권한 정책(`AdminOnly`, `OwnerOrAdmin`) 반영
- Step 1.5 완료 (API 문서화)
  - Swagger/OpenAPI 자동 생성 + `/swagger/index.html` 제공
- Step 2 완료 (도메인 확장)
  - 완료/반영
    - 사용자 suspension 조회/설정/해제 API
    - 일반 signup의 email 수집 + email 유니크 검증
    - 서버 발급 guest 계정 + guest -> 정식 계정 업그레이드 API
    - 이메일 인증 v1 (`request/confirm`, signup/upgrade 자동 발송, verified-only 기능 제한)
    - 비밀번호 재설정 v1 (`request/confirm`, 1회용 token, 전체 세션 무효화)
    - Post 상태(`draft/published/deleted`) + draft 생성/발행 API
    - post 검색 v1: `GET /api/v1/posts/search` + in-memory BM25 adapter (`title/content/tag`, whitespace tokenizer, all-terms-match, outbox event 기반 비동기 인덱싱)
    - Comment 상태(`active/deleted`) + 1-depth reply/tombstone 정책
    - Reaction `me` 기준 upsert/delete + target 단일 상태 보장
    - Tag 연계: post create/update + `GET /api/v1/tags/{tagName}/posts` 조회
    - Attachment 업로드/조회/preview/delete + `attachment://{id}` 참조 검증
    - attachment orphan/pending-delete + cleanup job 등록
    - Report 도메인(`post/comment` 신고, 중복 금지, admin 수동 처리)
    - admin 운영 API: 신고 목록/처리, 게시판 hidden 제어
    - 사용자 soft delete + 익명화 + 외부 식별자(`uuid`) 노출
    - guest의 post/comment create/update/delete 허용 + draft/publish/attachment/reaction/report 차단
    - 일반 미인증 사용자의 post/comment/reaction 허용 + attachment/report는 verified email 필요
    - guest lifecycle(`pending/active/expired`) + guest cleanup job
    - password reset v2
      - frontend reset 링크 메일 템플릿 + fallback token
      - `client_ip + normalized_email` 기준 request rate limit
      - expired/consumed token cleanup job + 구조화 audit log
    - Ranking v1
      - `GET /api/v1/posts/feed?sort=hot|best|latest`
      - `PublishedAt` 도입 + draft publish 시점 기준 시간축 정리
      - `reaction/comment` 기반 Hot/Best 점수 정책
      - `post.changed` / `comment.changed` / `reaction.changed` outbox relay 소비 기반 비동기 ranking read-side 갱신
    - Ranking v2
      - 공개 목록 전반 확장: `feed + board + tag + search`
      - 공통 정렬 규약
        - `feed`: `hot|best|latest|top`
        - `board`, `tag`: `latest|hot|best|top`
        - `search`: `relevance|hot|latest|top`
      - `top` + `window=24h|7d|30d|all` 도입
      - `search relevance`는 기존 BM25 유지, `hot|latest|top`은 검색 결과 집합 내부 재정렬
      - ranking read-side activity ledger를 `24h/7d/30d/all` window 계산으로 확장
    - email verification v2
      - frontend verification 링크 메일 템플릿 + fallback token
      - `user_id` 기준 request rate limit
      - expired/consumed verification token cleanup job + 구조화 audit log
  - 보류/후속
    - soft delete의 게시글/댓글 등 다른 도메인 확장 여부는 별도 결정으로 남긴다.
- Step 3 미완료 (보안/어뷰징 1차)
  - 완료/반영
    - `RateLimiter` 포트 + in-memory fixed-window 구현
    - `/api/v1` read/write 요청 IP 기준 `429` 제한
    - auth security hardening
      - `POST /api/v1/auth/login` 전용 rate limit (`client_ip + normalized_username`)
      - `POST /api/v1/auth/guest/upgrade` 전용 rate limit (`user_id + client_ip`)
      - `login_attempt`, `guest_upgrade_attempt` 구조화 audit log
    - markdown 본문/텍스트 입력은 raw 저장, 출력 렌더링 경계에서 XSS 방어 책임 유지
  - 미완료/후속
    - 클라이언트 마크다운 렌더링 XSS sanitization(DOMPurify + allowlist 적용)은 Step 8 Web UI에서 처리
- Step 4 완료 (EDA)
  - 완료/반영
    - 서비스 표준 발행 경로를 `tx 내부 outbox append`로 단일화
    - in-memory outbox store + relay worker + serializer
    - at-least-once 전달, retry/backoff/max-attempt/dead 정책
    - stale `processing` 메시지 reclaim(lease timeout) 복구 경로
    - cache invalidation을 relay 소비로 전환 (eventual consistency)
    - signal 기반 bounded graceful shutdown + relay drain 경계
    - dead 운영 정책 정리: requeue(dead->pending) / discard(삭제)
    - dead outbox admin/ops API: 목록/재처리/폐기
    - 서비스 이벤트 경계 용어 정리: `eventPublisher` -> `actionDispatcher`, outbox 우선 + fallback dispatch
    - Notification 도메인 v1
      - `/users/me/notifications`, unread count, 개별 읽음 처리
      - `post_commented`, `comment_replied`, `mentioned` 이벤트 적재
      - post/comment create 시 FE 명시 `mentioned_usernames` 기반 mention 적재
    - notification backend contract v2
      - 목록 응답에 `is_read`, `target_kind`, `message_key`, `message_args` 추가
      - `PATCH /users/me/notifications/read-all` 추가
      - typed target + localization-friendly message contract 정리
    - ranking read-side relay
      - `post.changed`, `comment.changed`, `reaction.changed` 소비
      - in-memory `PostRankingRepository` projection + opaque feed cursor
      - feed 관련 cache invalidation 경로
- Step 5 완료 (외부 인프라 전환)
  - SQLite foundation
    - `internal/infrastructure/persistence/sqlite` 추가
    - ordered `.sql` migration + `schema_migrations` ledger + WAL/foreign_keys pragmas
  - User/Auth adapter wiring
    - `UserRepository`, `EmailVerificationTokenRepository`, `PasswordResetTokenRepository`를 SQLite DB로 전환
    - `cfg.Database.Path` + `cmd/main.go` wiring 적용
  - Board/Post adapter implementation
    - `BoardRepository`, `TagRepository`, `PostTagRepository`, `PostRepository`, `PostSearchRepository`를 SQLite DB 기반으로 추가
    - repository contract + search ranking smoke test 통과
    - `cmd/main.go` wiring + tx-bound `sqlite.UnitOfWork` 반영
  - Outbox 내구화
    - `OutboxRepository`를 SQLite DB에 붙이고 relay/admin/tx append를 전환
  - SQLite repository/search/outbox 어댑터 점진 전환
    - comment/reaction/attachment/report/notification SQLite repository 전환 완료
    - Search FTS5 전환 완료: `post_search_fts` virtual table + 후보 필터/기존 Go ranking 유지
  - MQ bridge는 채택하지 않음
- Step 6 완료 (관측성/예외)
  - 완료/반영: 서비스 에러 래핑, HTTP 공개 에러 정규화, `slog` 구조화 로그, `lumberjack` 파일 로테이션, panic/recover 로깅
  - 내부 taxonomy를 `repository/cache/token/sqlite/mail/storage`로 세분화해 운영 추적성을 보강한다
- Step 7 미착수 (배포)
  - reverse proxy + TLS 종료 + 단일 바이너리 배포 파이프라인
- Step 8 완료 (Web UI / Admin Console)
  - `internal/delivery/web` 기반 HTML-first UI shell, cookie-first auth transport, SSR fallback metadata, current user summary, draft recovery/resume, admin console 계약이 구현됨
  - `/api/v1`는 JSON API 전용으로 유지하고, 브라우저 UI route space는 `/`, `/login`, `/me`, `/admin/...`로 분리됨
  - `GET /api/v1/users/me`, `GET /api/v1/users/me/drafts`, `GET /api/v1/posts/{postUUID}/draft` current-user / draft resume 계약이 반영됨
  - go:embed 단일 바이너리 정적 자산/템플릿 서빙, Cookie/Bearer 병행 미들웨어, CSRF double-submit cookie 방어가 반영됨
  - login/logout cookie 계약, `my_reaction_type` 조회 계약, email verification/password reset 진입 라우트 계약이 반영됨
  - 클라이언트 마크다운 XSS sanitization (DOMPurify + allowlist), guest lifecycle UI 흐름이 반영됨
  - Playwright `chromium` + `visual` e2e/snapshot 회귀 테스트로 주요 화면과 레이아웃이 고정됨

세부 구현 상태와 우선순위 변경 이력은 `docs/DECISIONS.md` 기준으로 관리한다.
