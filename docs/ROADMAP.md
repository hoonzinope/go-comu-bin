# go-comu-bin 로드맵

이 문서는 go-comu-bin의 개발 순서를 명확히 고정하기 위한 실행 로드맵입니다.
진행 중 예외 수정은 허용하지만, 기본 우선순위는 아래 순서를 따릅니다.

## 전체 방향

- Phase 1: 애플리케이션 코어 완성 (순수 Go, In-Memory 중심)
- Phase 2: 인프라스트럭처 어댑터 결합 (SQLite All-in-One)
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
  - PointHistory (원장/스냅샷)
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
  - SEO용 Slug 충돌 처리
  - Reaction 중복 방지 정책 도입
    - 동일 사용자의 리액션은 `target_id`/`target_type` 기준 단일 상태로 유지
    - `me` 기준 upsert/delete API를 통해 생성/변경/삭제를 target 중심으로 처리
    - SQLite 도입 시 `user_id`, `target_id`, `target_type` 복합 유니크 키로 데이터 무결성 보장
  - 동적 게이미피케이션 룰 엔진 설계
    - 예시: 글 작성 +10, 댓글 작성 +2
    - 하드코딩 대신 DB/Config 기반 규칙 조회 및 적용

### Step 3. 어뷰징 방지 및 보안 로직 도입

- Rate Limit 포트 정의 (도배 방지)
- Sanitizer 파이프라인 구축 (XSS 방어)

### Step 4. 이벤트 기반 아키텍처 도입

- Go Channel 기반 내부 Event Bus
- 비동기/분산 트랜잭션 안정성 강화를 위한 Outbox 포트 설계
- 플러그인 확장 뼈대(Hook System) 설계
  - Action Hook: 코어 이벤트 발행/구독 확장 포인트
  - Filter Hook: 데이터 가공 파이프라인 확장 포인트
  - 확장 메모: 추후 WebAssembly/스크립트 엔진 연계를 고려한 인터페이스 우선 설계
- 멘션(Mention) 이벤트 유스케이스 추가
  - 본문/댓글의 `@username` 파싱 -> `MentionedEvent` 발행
  - Notification 도메인과 연결해 재방문 유도 흐름 구축

## Phase 2. 인프라스트럭처 어댑터 결합 (SQLite All-in-One)

Phase 1에서 정의한 포트에 대해, 외부 의존 최소화 원칙으로 어댑터를 결합합니다.

### Step 5. 외부 인프라 어댑터 일괄 교체

- RDB Repository: SQLite 어댑터 (WAL 모드)
- Search: SQLite FTS5 기반 전문 검색
- Message Queue: SQLite Outbox 테이블 폴링
- Object Storage: Local File System (파일은 디스크, 메타데이터는 DB)
- Cache: Ristretto 또는 고성능 인메모리 캐시 구현체
  - 캐시 무효화 전략 개선
    - 현재 서비스 계층 수동 무효화 방식에서 발생하는 불일치 리스크를 줄이기 위해, 이벤트 기반 무효화 또는 중앙 집중식 캐시 정책으로 전환
    - 캐시 쓰기/삭제 책임을 분산시키지 않도록 공통 계층 또는 이벤트 소비 지점으로 수렴
- Webhook 발송 매니저
  - Outbox 이벤트를 외부 HTTP URL(Discord/Slack/사용자 서버)로 비동기 POST
  - 실패 재시도(백오프), dead-letter 기준, 전달 보장 정책 정의

## Phase 3. 프로덕션 고도화 및 배포

### Step 6. 관측성(Observability) 및 예외 처리 고도화

- `log/slog` + `lumberjack` 기반 JSON 로그 로테이션
- 글로벌 커스텀 에러 핸들러
- 에러 핸들링 전략 세분화 및 로깅 강화
  - `customError.ErrInternalServerError`에 과도하게 수렴하는 흐름을 줄이고, DB/캐시/외부 연동 등 원인별 에러 타입을 구체화
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
- `AccountUseCase`를 통해 계정 삭제와 세션 무효화 orchestration 분리
- 사용자 탈퇴는 soft delete + 익명화 정책으로 전환
  - 기존 username/password는 재사용 불가 상태로 비식별화
  - 세션 정리는 best effort로 유지
- 사용자 공개 식별자로 `uuid` 도입
  - 내부 PK/FK는 `int64` 유지
  - post/comment/reaction 응답은 `author_uuid`, `user_uuid` 노출
- Reaction API를 `me` 기준 upsert/delete로 정리
  - `PUT /posts/{id}/reactions/me`
  - `DELETE /posts/{id}/reactions/me`
  - `PUT /comments/{id}/reactions/me`
  - `DELETE /comments/{id}/reactions/me`
- 저장소 contract test 확대
  - `ReactionRepository`
  - `UserRepository`
  - `BoardRepository`

남은 작업

- `lumberjack` 기반 파일 로테이션
- panic/미들웨어 수준 글로벌 예외 로깅 정교화
- SQLite/외부 어댑터 도입 후 원인별 에러 타입 추가 세분화

### Step 7. 배포 환경 구축

- Caddy 리버스 프록시 + 자동 HTTPS(SSL)
- 단일 실행 파일(`bin/commu-bin`) + `data.db` 배포
- 대상: M4 Mac mini 홈 서버

## 운영 관점 우선순위 메모

- 위 확장 항목(이메일 인증/비밀번호 재설정, 태그, 멘션, 자동 백업)은 상용 서비스 안정성을 위해 중요도가 높음
- 단, 초기 개발 속도를 위해 Phase 1의 기존 핵심 경로를 먼저 완주한 뒤 Step 2와 4 내 세부 태스크로 병행 착수 가능

## 현재 상태 메모

- Step 0, Step 1의 핵심 토대는 이미 반영되어 있음
  - Config 로딩/검증
  - JWT 인증 middleware + role 기반 인가 정책
- Step 1.5는 반영됨
  - Swagger/OpenAPI 자동 생성 + `/swagger/index.html` 제공
- Step 2 일부는 반영됨
  - Reaction 중복 방지/변경 정책
  - 사용자 soft delete + 익명화
  - 사용자 uuid 공개 식별자 도입
- Step 6 일부는 반영됨
  - 서비스 에러 래핑
  - HTTP 에러 정규화
  - `slog` 기반 구조화 에러 로그
- 세부 구현 상태는 작업 이슈/PR 단위로 관리
