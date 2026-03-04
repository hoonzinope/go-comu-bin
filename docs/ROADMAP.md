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

### Step 1. 인증/인가(Auth) 미들웨어 적용

- 목표: 보호 라우트의 인증/인가 경계 확립
- JWT 토큰 검증
- Context를 통한 `user_id` 전달
- role 기반 권한 정책 적용

### Step 2. 비즈니스 고도화 및 도메인 확장

- 신규 도메인
  - Attachment (파일 메타)
  - Report (신고)
  - Notification (알림)
  - PointHistory (원장/스냅샷)
- 운영/제재 규칙
  - 유저 정지(Suspension)
  - 게시물 임시저장(Draft)
  - Soft Delete
- 구조 개선
  - Offset -> Cursor 페이지네이션으로 DTO/Port 재설계
  - SEO용 Slug 충돌 처리

### Step 3. 어뷰징 방지 및 보안 로직 도입

- Rate Limit 포트 정의 (도배 방지)
- Sanitizer 파이프라인 구축 (XSS 방어)

### Step 4. 이벤트 기반 아키텍처 도입

- Go Channel 기반 내부 Event Bus
- 비동기/분산 트랜잭션 안정성 강화를 위한 Outbox 포트 설계

## Phase 2. 인프라스트럭처 어댑터 결합 (SQLite All-in-One)

Phase 1에서 정의한 포트에 대해, 외부 의존 최소화 원칙으로 어댑터를 결합합니다.

### Step 5. 외부 인프라 어댑터 일괄 교체

- RDB Repository: SQLite 어댑터 (WAL 모드)
- Search: SQLite FTS5 기반 전문 검색
- Message Queue: SQLite Outbox 테이블 폴링
- Object Storage: Local File System (파일은 디스크, 메타데이터는 DB)
- Cache: Ristretto 또는 고성능 인메모리 캐시 구현체

## Phase 3. 프로덕션 고도화 및 배포

### Step 6. 관측성(Observability) 및 예외 처리 고도화

- `log/slog` + `lumberjack` 기반 JSON 로그 로테이션
- 글로벌 커스텀 에러 핸들러

### Step 7. 배포 환경 구축

- Caddy 리버스 프록시 + 자동 HTTPS(SSL)
- 단일 실행 파일(`bin/commu-bin`) + `data.db` 배포
- 대상: M4 Mac mini 홈 서버

## 현재 상태 메모

- Step 0, Step 1의 핵심 토대는 이미 반영되어 있음
  - Config 로딩/검증
  - JWT 인증 middleware + role 기반 인가 정책
- 세부 구현 상태는 작업 이슈/PR 단위로 관리
