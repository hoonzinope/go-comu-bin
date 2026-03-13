package port

// FilterHook는 유스케이스 실행 전/중 입력 검증/변환 파이프라인 확장 포인트다.
// 구체 계약은 도메인별 hook 도입 단계에서 확정한다.
type FilterHook interface{}

// ActionHookDispatcher는 유스케이스 실행 후 액션 훅을 트리거하는 경계다.
// Outbox 전달 경계와 분리된 확장 포인트로 유지한다.
type ActionHookDispatcher interface {
	Dispatch(events ...DomainEvent)
}
