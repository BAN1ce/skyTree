package plugin

type ClientOnlineState func(clientID string, online bool)

type OnClientOnline func(clientID string)

type OnClientOffline func(clientID string)

type ClientPlugin struct {
	OnClientOnline  []OnClientOnline
	OnClientOffline []OnClientOffline
}
