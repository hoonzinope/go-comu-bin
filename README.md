# go-comu-bin

Go로 작성한 single-binary 커뮤니티 엔진입니다.

이 저장소는 기능 완성형 제품보다, 아키텍처/도메인 설계를 학습하고 확장 가능한 형태로 발전시키는 데 초점을 둡니다.

## 빠른 시작

```bash
make run
```

또는:

```bash
go run ./cmd
```

기본 설정은 루트의 `config.yml`을 사용합니다.
실행 전 `delivery.http.auth.secret`에는 실제 비밀값을 넣어야 합니다.

Swagger UI는 서버 실행 후 [http://localhost:18577/swagger/index.html](http://localhost:18577/swagger/index.html) 에서 확인할 수 있습니다.

## 문서

상세 문서는 `docs/`로 분리되어 있습니다.

- [로드맵](docs/ROADMAP.md)
- [아키텍처](docs/ARCHITECTURE.md)
- [결정 기록](docs/DECISIONS.md)
- [설정](docs/CONFIG.md)
- [HTTP API](docs/API.md)
- [테스트 가이드](docs/TESTING.md)

## 현재 구현 요약

- Delivery: `gin` 기반 HTTP 어댑터
- 인증: `SessionUseCase` 기반 JWT + `SessionRepository` 세션 검증
- 인가: 주입 가능한 `AuthorizationPolicy` 기반(role/owner)
- 비밀번호: `PasswordHasher` 포트 기반 bcrypt 해시 저장/비교
- 사용자 식별자: 내부 `int64` + 외부 노출용 `uuid` 병행
- 데이터 저장소: In-Memory 어댑터
- 조회 캐시: 서비스 레이어 정책 + 캐시 포트(`GetOrSetWithTTL`, `DeleteByPrefix`)
- 페이지네이션: `limit + last_id` 커서 기반
- 모델 경계: `domain/entity` -> `application/model` -> `delivery/response`
- 포트 구성: `internal/application/port`, 매핑: `internal/application/mapper`
- API 규칙/스펙/운영 가이드는 `docs/` 문서를 단일 기준으로 관리

## 개발 명령

```bash
make help
make build
make run
make swagger
make verify
make test
```

Swagger 정합성 검증은 일상 개발 루프에 강제하지 않고, 명시적 품질 게이트인 `make verify`에만 포함합니다.
선택적으로 로컬 pre-commit 훅을 설치하려면 `./scripts/install-githooks.sh`를 실행하면 됩니다.
