package core

func NativeStreamBase() string {
	nativeState.Lock()
	engine := nativeState.engine
	nativeState.Unlock()
	if engine == nil {
		return ""
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.stream == nil {
		return ""
	}
	return engine.stream.address
}
