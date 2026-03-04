# 아키텍처

## 핵심 원칙

- Layered Architecture
- Port & Adapter (Hexagonal)
- Domain 중심 설계
- interface/implementation 분리

## 요청 흐름

`HTTP Delivery -> UseCase Port -> Service -> Repository Port -> InMemory Adapter`

## 인증/인가 흐름

- 인증
  - `gin` middleware가 `Authorization` 헤더에서 토큰 추출
  - JWT 검증 후 `context.user_id` 주입
  - token cache 조회로 유효 세션 확인
- 인가
  - Service 레이어에서 `AuthorizationPolicy`로 권한 판정
  - 기본 정책: `AdminOnly`, `OwnerOrAdmin`

## 세션 유효성 흐름

- login: 토큰 발급 후 cache 저장
- protected route: JWT 검증 + cache 확인
- logout: cache에서 토큰 삭제(무효화)

## 구성 루트 (Composition Root)

- 파일: `cmd/main.go`
- 역할
  - config 로딩
  - repository/usecase/auth/cache 조립
  - HTTP 서버 시작
  - admin 계정 시드(`admin/admin`)

## 디렉토리 구조

```txt
cmd/
  main.go

internal/
  delivery/
    http.go
    middleware/
      authMiddleware.go
    response/
      types.go
      mapper.go

  application/
    authentication.go
    cache.go
    useCase.go
    repository.go
    policy/
      authorization_policy.go
      role_authorization_policy.go
    service/
      *.go

  domain/
    entity/
      *.go
    dto/
      *.go

  infrastructure/
    auth/
      JwtTokenProvider.go
    cache/
      inmemory/
        in_memory_cache.go
    persistence/
      inmemory/
        *.go

  customError/
    customError.go
```

## 모델 분리 원칙

- `domain/entity`: 비즈니스 모델(직렬화 관심사 없음)
- `domain/dto`: 유스케이스 반환 모델
- `delivery/response`: HTTP 응답 스키마(JSON 태그 정의)

도메인 엔티티에는 `json` 태그를 두지 않고, 전달 계층에서 응답 모델로 매핑합니다.
