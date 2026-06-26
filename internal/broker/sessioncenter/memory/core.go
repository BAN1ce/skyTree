package memory

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"google.golang.org/protobuf/proto"
)

const neverExpireSessionInterval = ^uint32(0)

type Core struct {
	sessions *proto_session.ClusterSessions
	mux      sync.RWMutex
}

func NewCore() *Core {
	return &Core{
		sessions: &proto_session.ClusterSessions{
			Sessions: make(map[string]*proto_session.Session),
			Owners:   make(map[string]*proto_session.SessionOwner),
		},
	}
}

func (s *Core) OpenSessionForConnect(ctx context.Context, request *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.openSessionForConnect(request)
}

func (s *Core) TakeOverSessionOwner(ctx context.Context, request *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.takeOverSessionOwner(request)
}

func (s *Core) ReplaceSessionStateOnCleanStart(ctx context.Context, request *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.replaceSessionStateOnCleanStart(request)
}

func (s *Core) SaveOfflineState(ctx context.Context, request *proto_session.SaveOfflineStateRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.saveOfflineState(request)
}

func (s *Core) RemoveOutgoingUnfinished(ctx context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.removeOutgoingUnfinished(request)
}

func (s *Core) UpsertIncomingUnfinished(ctx context.Context, request *proto_session.UpsertIncomingUnfinishedRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.upsertIncomingUnfinished(request)
}

func (s *Core) RemoveIncomingUnfinished(ctx context.Context, request *proto_session.RemoveIncomingUnfinishedRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.removeIncomingUnfinished(request)
}

func (s *Core) CommitOutgoingProgress(ctx context.Context, request *proto_session.CommitOutgoingProgressRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.commitOutgoingProgress(request)
}

func (s *Core) DeleteSession(ctx context.Context, request *proto_session.DeleteSessionRequest) error {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.deleteSession(request)
}

func (s *Core) GetSession(ctx context.Context, request *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	_ = ctx
	s.mux.RLock()
	defer s.mux.RUnlock()

	session := s.sessions.GetSessions()[request.GetClientID()]
	if sessionExpired(session, requestNowUnixNano(request.GetNowUnixNano())) {
		session = nil
	}
	return &proto_session.ReadSessionResponse{
		Session: cloneSession(session),
		Exist:   session != nil,
	}, nil
}

func (s *Core) GetSessionOwner(ctx context.Context, request *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	_ = ctx
	s.mux.RLock()
	defer s.mux.RUnlock()

	owner := s.sessions.GetOwners()[request.GetClientID()]
	return &proto_session.ReadSessionOwnerResponse{
		Owner: cloneOwner(owner),
		Exist: owner != nil,
	}, nil
}

func (s *Core) GetSessionOwners(ctx context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	_ = ctx
	if len(request.GetClientIDs()) == 0 {
		return &proto_session.ReadSessionOwnersResponse{
			Items: []*proto_session.ReadSessionOwnerItem{},
		}, nil
	}

	s.mux.RLock()
	defer s.mux.RUnlock()

	seen := make(map[string]struct{}, len(request.GetClientIDs()))
	items := make([]*proto_session.ReadSessionOwnerItem, 0, len(request.GetClientIDs()))
	for _, clientID := range request.GetClientIDs() {
		if clientID == "" {
			continue
		}
		if _, ok := seen[clientID]; ok {
			continue
		}
		seen[clientID] = struct{}{}

		owner := s.sessions.GetOwners()[clientID]
		items = append(items, &proto_session.ReadSessionOwnerItem{
			ClientID: clientID,
			Owner:    cloneOwner(owner),
			Exist:    owner != nil,
		})
	}

	return &proto_session.ReadSessionOwnersResponse{
		Items: items,
	}, nil
}

func (s *Core) DeleteExpiredSessions(ctx context.Context, nowUnixNano int64) ([]string, error) {
	_ = ctx
	s.mux.Lock()
	defer s.mux.Unlock()

	now := requestNowUnixNano(nowUnixNano)
	deleted := make([]string, 0)
	for clientID, sessionState := range s.sessions.GetSessions() {
		if !sessionExpired(sessionState, now) {
			continue
		}
		delete(s.sessions.Sessions, clientID)
		delete(s.sessions.Owners, clientID)
		deleted = append(deleted, clientID)
	}
	return deleted, nil
}

func (s *Core) openSessionForConnect(request *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	clientID := request.GetClientID()
	if clientID == "" {
		return nil, fmt.Errorf("client id is required")
	}

	oldSession := s.sessions.GetSessions()[clientID]
	now := requestNowUnixNano(request.GetNowUnixNano())
	if sessionExpired(oldSession, now) {
		delete(s.sessions.Sessions, clientID)
		delete(s.sessions.Owners, clientID)
		oldSession = nil
	}
	var effective *proto_session.Session
	if oldSession != nil {
		effective = cloneSession(oldSession)
	} else {
		effective = &proto_session.Session{ClientID: clientID}
	}
	effective.ClientID = clientID
	effective.WillMessage = cloneWillMessage(request.GetWillMessage())
	effective.SessionExpiryInterval = request.GetSessionExpiryInterval()
	effective.ExpireAtUnixNano = 0

	s.sessions.Sessions[clientID] = effective
	logger.Logger.Debug().Str("client", clientID).Msg("opened session for connect")

	return &proto_session.OpenSessionForConnectResponse{
		OldSession:       cloneSession(oldSession),
		OldSessionExists: oldSession != nil,
		Session:          cloneSession(effective),
	}, nil
}

func (s *Core) takeOverSessionOwner(request *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	if request == nil || request.GetOwner() == nil {
		return nil, fmt.Errorf("owner is required")
	}
	clientID := request.GetOwner().GetClientID()
	if clientID == "" {
		return nil, fmt.Errorf("client id is required")
	}

	oldOwner := s.sessions.GetOwners()[clientID]
	newOwner := cloneOwner(request.GetOwner())
	s.sessions.Owners[clientID] = newOwner
	logger.Logger.Debug().Str("client", clientID).Uint64("node", newOwner.GetNodeID()).Bool("online", newOwner.GetOnline()).Msg("took over session owner")

	return &proto_session.TakeOverSessionOwnerResponse{
		PreviousOwner:       cloneOwner(oldOwner),
		PreviousOwnerExists: oldOwner != nil,
		Owner:               cloneOwner(newOwner),
	}, nil
}

func (s *Core) replaceSessionStateOnCleanStart(request *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	clientID := request.GetClientID()
	if clientID == "" {
		return fmt.Errorf("client id is required")
	}

	s.sessions.Sessions[clientID] = &proto_session.Session{
		ClientID:              clientID,
		WillMessage:           cloneWillMessage(request.GetWillMessage()),
		UnfinishedMessages:    nil,
		SessionExpiryInterval: request.GetSessionExpiryInterval(),
		ExpireAtUnixNano:      0,
	}
	logger.Logger.Debug().Str("client", clientID).Msg("replaced session state on clean start")
	return nil
}

func (s *Core) saveOfflineState(request *proto_session.SaveOfflineStateRequest) error {
	clientID := request.GetClientID()
	if clientID == "" {
		return fmt.Errorf("client id is required")
	}
	if ownerToken := request.GetOwnerToken(); ownerToken != "" {
		owner := s.sessions.GetOwners()[clientID]
		if owner != nil && owner.GetOwnerToken() != ownerToken {
			logger.Logger.Warn().
				Str("client", clientID).
				Str("request_owner_token", ownerToken).
				Str("current_owner_token", owner.GetOwnerToken()).
				Msg("ignored stale offline session state")
			return nil
		}
	}

	sessionState := s.sessions.GetSessions()[clientID]
	sessionExpiryInterval := request.GetSessionExpiryInterval()
	if sessionExpiryInterval == 0 {
		return s.deleteSession(&proto_session.DeleteSessionRequest{ClientID: clientID})
	}
	if sessionState == nil {
		sessionState = &proto_session.Session{ClientID: clientID}
	}
	sessionState.ClientID = clientID
	sessionState.UnfinishedMessages = mergeOfflineUnfinishedMessages(
		sessionState.GetUnfinishedMessages(),
		request.GetUnfinishedMessages(),
	)
	sessionState.OutgoingReplayCursor = cloneOutgoingReplayCursor(request.GetOutgoingReplayCursor())
	sessionState.SessionExpiryInterval = sessionExpiryInterval
	sessionState.ExpireAtUnixNano = sessionExpiryDeadlineUnixNano(sessionExpiryInterval, requestNowUnixNano(request.GetNowUnixNano()))
	switch {
	case request.GetClearWill():
		sessionState.WillMessage = nil
	case request.GetWillMessage() != nil:
		sessionState.WillMessage = cloneWillMessage(request.GetWillMessage())
	}
	s.sessions.Sessions[clientID] = sessionState

	if owner := s.sessions.GetOwners()[clientID]; owner != nil {
		owner = cloneOwner(owner)
		owner.Online = false
		s.sessions.Owners[clientID] = owner
	}

	logger.Logger.Debug().
		Str("client", clientID).
		Int("unfinished_count", len(sessionState.GetUnfinishedMessages())).
		Bool("has_outgoing_replay_cursor", sessionState.GetOutgoingReplayCursor() != nil).
		Bool("clear_will", request.GetClearWill()).
		Msg("saved offline session state")
	return nil
}

func (s *Core) removeOutgoingUnfinished(request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	clientID := request.GetClientID()
	if clientID == "" {
		return fmt.Errorf("client id is required")
	}
	messageID := request.GetMessageID()
	if messageID == "" {
		return fmt.Errorf("message id is required")
	}

	sessionState := s.sessions.GetSessions()[clientID]
	if sessionExpired(sessionState, requestNowUnixNano(request.GetNowUnixNano())) {
		delete(s.sessions.Sessions, clientID)
		delete(s.sessions.Owners, clientID)
		return nil
	}
	if sessionState == nil || len(sessionState.GetUnfinishedMessages()) == 0 {
		return nil
	}

	kept := make([]*proto_session.UnfinishedMessage, 0, len(sessionState.GetUnfinishedMessages()))
	for _, m := range sessionState.GetUnfinishedMessages() {
		if m == nil {
			continue
		}
		if m.GetIsOutgoing() && m.GetMessageID() == messageID {
			continue
		}
		kept = append(kept, proto.Clone(m).(*proto_session.UnfinishedMessage))
	}
	sessionState = cloneSession(sessionState)
	sessionState.UnfinishedMessages = kept
	s.sessions.Sessions[clientID] = sessionState
	return nil
}

// commitOutgoingProgress updates only the outgoing delivery progress of an online session:
// the QoS1 replay cursor and the outgoing (QoS2/retained) unfinished messages. It preserves
// incoming unfinished messages, the will, session expiry, and owner online state, so it is
// safe to call periodically while the client is connected. It never creates a session.
func (s *Core) commitOutgoingProgress(request *proto_session.CommitOutgoingProgressRequest) error {
	clientID := request.GetClientID()
	if clientID == "" {
		return fmt.Errorf("client id is required")
	}
	if !s.ownerTokenMatches(clientID, request.GetOwnerToken(), "ignored stale outgoing progress commit") {
		return nil
	}

	sessionState := s.sessions.GetSessions()[clientID]
	if sessionExpired(sessionState, requestNowUnixNano(request.GetNowUnixNano())) {
		delete(s.sessions.Sessions, clientID)
		delete(s.sessions.Owners, clientID)
		return nil
	}
	if sessionState == nil {
		// No persistent session to update (e.g. clean session). Nothing to commit.
		return nil
	}

	sessionState = cloneSession(sessionState)
	sessionState.ClientID = clientID
	// Replace only the outgoing portion; reuse the offline merge which keeps incoming entries
	// and overwrites outgoing ones with the supplied (outgoing-only) snapshot.
	sessionState.UnfinishedMessages = mergeOfflineUnfinishedMessages(
		sessionState.GetUnfinishedMessages(),
		request.GetUnfinishedMessages(),
	)
	sessionState.OutgoingReplayCursor = cloneOutgoingReplayCursor(request.GetOutgoingReplayCursor())
	s.sessions.Sessions[clientID] = sessionState
	return nil
}

func (s *Core) upsertIncomingUnfinished(request *proto_session.UpsertIncomingUnfinishedRequest) error {
	clientID := request.GetClientID()
	if clientID == "" {
		return fmt.Errorf("client id is required")
	}
	packetID := request.GetPacketID()
	if packetID == 0 {
		return fmt.Errorf("packet id is required")
	}
	if request.GetUnfinishedMessage() == nil {
		return fmt.Errorf("unfinished message is required")
	}
	if !s.ownerTokenMatches(clientID, request.GetOwnerToken(), "ignored stale incoming unfinished upsert") {
		return nil
	}

	sessionState := s.sessions.GetSessions()[clientID]
	if sessionExpired(sessionState, requestNowUnixNano(request.GetNowUnixNano())) {
		delete(s.sessions.Sessions, clientID)
		delete(s.sessions.Owners, clientID)
		sessionState = nil
	}
	if sessionState == nil {
		sessionState = &proto_session.Session{ClientID: clientID}
	}

	unfinished := proto.Clone(request.GetUnfinishedMessage()).(*proto_session.UnfinishedMessage)
	unfinished.PacketID = packetID
	unfinished.Qos = 2
	unfinished.State = proto_session.UnfinishedMessage_WAITING_PUBREL
	unfinished.IsOutgoing = false

	sessionState = cloneSession(sessionState)
	sessionState.ClientID = clientID
	sessionState.UnfinishedMessages = upsertIncomingUnfinishedMessage(sessionState.GetUnfinishedMessages(), unfinished)
	s.sessions.Sessions[clientID] = sessionState
	return nil
}

func (s *Core) removeIncomingUnfinished(request *proto_session.RemoveIncomingUnfinishedRequest) error {
	clientID := request.GetClientID()
	if clientID == "" {
		return fmt.Errorf("client id is required")
	}
	packetID := request.GetPacketID()
	if packetID == 0 {
		return fmt.Errorf("packet id is required")
	}
	if !s.ownerTokenMatches(clientID, request.GetOwnerToken(), "ignored stale incoming unfinished removal") {
		return nil
	}

	sessionState := s.sessions.GetSessions()[clientID]
	if sessionExpired(sessionState, requestNowUnixNano(request.GetNowUnixNano())) {
		delete(s.sessions.Sessions, clientID)
		delete(s.sessions.Owners, clientID)
		return nil
	}
	if sessionState == nil || len(sessionState.GetUnfinishedMessages()) == 0 {
		return nil
	}

	kept := make([]*proto_session.UnfinishedMessage, 0, len(sessionState.GetUnfinishedMessages()))
	for _, m := range sessionState.GetUnfinishedMessages() {
		if m == nil {
			continue
		}
		if !m.GetIsOutgoing() && m.GetPacketID() == packetID {
			continue
		}
		kept = append(kept, proto.Clone(m).(*proto_session.UnfinishedMessage))
	}
	sessionState = cloneSession(sessionState)
	sessionState.UnfinishedMessages = kept
	s.sessions.Sessions[clientID] = sessionState
	return nil
}

func (s *Core) ownerTokenMatches(clientID, ownerToken, staleMessage string) bool {
	if ownerToken == "" {
		return true
	}
	owner := s.sessions.GetOwners()[clientID]
	if owner == nil || owner.GetOwnerToken() == ownerToken {
		return true
	}
	logger.Logger.Warn().
		Str("client", clientID).
		Str("request_owner_token", ownerToken).
		Str("current_owner_token", owner.GetOwnerToken()).
		Msg(staleMessage)
	return false
}

func requestNowUnixNano(now int64) int64 {
	if now > 0 {
		return now
	}
	return time.Now().UnixNano()
}

func sessionExpired(session *proto_session.Session, now int64) bool {
	if session == nil {
		return false
	}
	expireAt := session.GetExpireAtUnixNano()
	return expireAt > 0 && now >= expireAt
}

func sessionExpiryDeadlineUnixNano(interval uint32, now int64) int64 {
	if interval == 0 || interval == neverExpireSessionInterval {
		return 0
	}
	return now + int64(interval)*int64(time.Second)
}

func (s *Core) deleteSession(request *proto_session.DeleteSessionRequest) error {
	clientID := request.GetClientID()
	logger.Logger.Debug().Str("client", clientID).Msg("delete session")
	delete(s.sessions.Sessions, clientID)
	delete(s.sessions.Owners, clientID)
	return nil
}

// WriteSnapshot serializes current sessions into writer.
func (s *Core) WriteSnapshot(writer io.Writer) error {
	s.mux.RLock()
	defer s.mux.RUnlock()

	data, err := proto.Marshal(s.sessions)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

// RecoverSnapshot restores sessions from reader.
func (s *Core) RecoverSnapshot(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	s.mux.Lock()
	defer s.mux.Unlock()

	s.sessions = &proto_session.ClusterSessions{}
	if err := proto.Unmarshal(data, s.sessions); err != nil {
		return err
	}
	if s.sessions.Sessions == nil {
		s.sessions.Sessions = make(map[string]*proto_session.Session)
	}
	if s.sessions.Owners == nil {
		s.sessions.Owners = make(map[string]*proto_session.SessionOwner)
	}
	return nil
}

func cloneSession(in *proto_session.Session) *proto_session.Session {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*proto_session.Session)
}

func cloneOwner(in *proto_session.SessionOwner) *proto_session.SessionOwner {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*proto_session.SessionOwner)
}

func cloneWillMessage(in *proto_session.WillMessage) *proto_session.WillMessage {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*proto_session.WillMessage)
}

func cloneOutgoingReplayCursor(in *proto_session.OutgoingReplayCursor) *proto_session.OutgoingReplayCursor {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*proto_session.OutgoingReplayCursor)
}

func cloneUnfinishedMessages(in []*proto_session.UnfinishedMessage) []*proto_session.UnfinishedMessage {
	if in == nil {
		return nil
	}
	out := make([]*proto_session.UnfinishedMessage, 0, len(in))
	for _, msg := range in {
		if msg == nil {
			continue
		}
		out = append(out, proto.Clone(msg).(*proto_session.UnfinishedMessage))
	}
	return out
}

func mergeOfflineUnfinishedMessages(
	existing []*proto_session.UnfinishedMessage,
	offline []*proto_session.UnfinishedMessage,
) []*proto_session.UnfinishedMessage {
	out := make([]*proto_session.UnfinishedMessage, 0, len(existing)+len(offline))
	offlineIncoming := make(map[uint32]struct{})
	for _, msg := range offline {
		if msg == nil || msg.GetIsOutgoing() {
			continue
		}
		offlineIncoming[msg.GetPacketID()] = struct{}{}
	}
	for _, msg := range existing {
		if msg == nil || msg.GetIsOutgoing() {
			continue
		}
		if _, replaced := offlineIncoming[msg.GetPacketID()]; replaced {
			continue
		}
		out = append(out, proto.Clone(msg).(*proto_session.UnfinishedMessage))
	}
	for _, msg := range offline {
		if msg == nil {
			continue
		}
		out = append(out, proto.Clone(msg).(*proto_session.UnfinishedMessage))
	}
	return out
}

func upsertIncomingUnfinishedMessage(
	messages []*proto_session.UnfinishedMessage,
	unfinished *proto_session.UnfinishedMessage,
) []*proto_session.UnfinishedMessage {
	out := make([]*proto_session.UnfinishedMessage, 0, len(messages)+1)
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if !msg.GetIsOutgoing() && msg.GetPacketID() == unfinished.GetPacketID() {
			continue
		}
		out = append(out, proto.Clone(msg).(*proto_session.UnfinishedMessage))
	}
	out = append(out, proto.Clone(unfinished).(*proto_session.UnfinishedMessage))
	return out
}
