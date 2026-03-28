package port

// ActionHookDispatcher는 유스케이스 실행 후 액션 이벤트를 전달하는 경계다.
// Outbox 전달 경계와 분리된 좁은 확장 포인트로 유지한다.
type ActionHookDispatcher interface {
	Dispatch(events ...DomainEvent)
}
