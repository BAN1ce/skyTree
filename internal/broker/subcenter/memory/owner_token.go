package memory

// getOwnerToken 读取客户端当前 owner token，空字符串表示没有绑定。
func (t *MemorySubCenter) getOwnerToken(clientID string) string {
	if clientID == "" {
		return ""
	}
	t.ownerMux.RLock()
	defer t.ownerMux.RUnlock()
	return t.clientOwnerTokens[clientID]
}

// setOwnerToken 设置客户端 owner token；空 token 表示清除绑定。
func (t *MemorySubCenter) setOwnerToken(clientID, token string) {
	if clientID == "" {
		return
	}
	t.ownerMux.Lock()
	defer t.ownerMux.Unlock()
	if token == "" {
		delete(t.clientOwnerTokens, clientID)
		return
	}
	t.clientOwnerTokens[clientID] = token
}

// copyOwnerTokens 复制 owner token map，用于快照序列化。
func (t *MemorySubCenter) copyOwnerTokens() map[string]string {
	t.ownerMux.RLock()
	defer t.ownerMux.RUnlock()
	out := make(map[string]string, len(t.clientOwnerTokens))
	for k, v := range t.clientOwnerTokens {
		out[k] = v
	}
	return out
}
