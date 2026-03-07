# [Refactor] 에러 핸들링 전략 세분화 및 로깅 강화 (Error Handling Refinement)

### 1. 현재 문제점
- 현재 많은 서비스 로직에서 `customError.ErrInternalServerError`를 광범위하게 사용하고 있습니다.
- 구체적인 에러 원인(데이터베이스 에러, 캐시 에러 등)이 마스킹되어 디버깅 시 실제 원인을 파악하기 어렵습니다.
- 에러 발생 시 로그를 남기는 로직이 부족하여 운영 환경에서 모니터링이 어렵습니다.

### 2. 개선 제안
- **에러 세분화**: `internal/customError`에 더 구체적인 에러 타입을 정의하고, 상황에 맞는 에러를 반환하도록 수정합니다. (예: `ErrDatabaseConnection`, `ErrCacheOperationFail` 등)
- **에러 래핑(Error Wrapping)**: `fmt.Errorf("...: %w", err)`를 사용하여 원본 에러 정보를 유지하면서 컨텍스트를 추가합니다.
- **중앙 집중식 로깅**: `Global Error Handler` 또는 서비스 레이어에서 `slog` 등을 활용하여 에러 로그를 체계적으로 기록합니다. (Roadmap Step 6 연계)

### 3. 기대 효과
- 빠른 문제 원인 파악 및 디버깅 생산성 향상.
- 시스템 안정성 및 관측성(Observability) 강화.