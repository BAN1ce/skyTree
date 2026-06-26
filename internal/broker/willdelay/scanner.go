package willdelay

import (
	"context"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
)

// Scanner 扫描器，只有 Leader 节点运行
type Scanner struct {
	ctx             context.Context
	cancel          context.CancelFunc
	center          Center
	sessionCenter   session.Center
	willMessageChan chan<- *brokerpublish.Message
	cluster         leaderReader
	clusterID       uint64
	localNodeID     uint64

	scanInterval time.Duration

	mux     sync.Mutex
	running bool
}

type leaderReader interface {
	GetLeader(clusterID uint64) (uint64, bool, error)
}

// NewScanner 创建新的扫描器
func NewScanner(
	ctx context.Context,
	center Center,
	sessionCenter session.Center,
	willMessageChan chan<- *brokerpublish.Message,
	cluster *raft.Cluster,
	clusterID uint64,
	localNodeID uint64,
) *Scanner {
	scannerCtx, cancel := context.WithCancel(ctx)
	scanner := &Scanner{
		ctx:             scannerCtx,
		cancel:          cancel,
		center:          center,
		sessionCenter:   sessionCenter,
		willMessageChan: willMessageChan,
		clusterID:       clusterID,
		localNodeID:     localNodeID,
		scanInterval:    1 * time.Second,
		running:         false,
	}
	if cluster != nil {
		scanner.cluster = cluster
	}
	return scanner
}

// Start 启动扫描器
func (s *Scanner) Start() {
	s.mux.Lock()
	if s.running {
		s.mux.Unlock()
		return
	}
	s.running = true
	s.mux.Unlock()

	go s.run()
}

// Stop 停止扫描器
func (s *Scanner) Stop() {
	s.mux.Lock()
	defer s.mux.Unlock()

	if !s.running {
		return
	}

	s.running = false
	s.cancel()
}

// run 运行扫描循环
func (s *Scanner) run() {
	scanTicker := time.NewTicker(s.scanInterval)
	defer scanTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Logger.Info().Msg("will delay scanner stopped")
			return

		case <-scanTicker.C:
			if !s.isCurrentLeader() {
				continue
			}
			s.scanAndExecute()
		}
	}
}

func (s *Scanner) isCurrentLeader() bool {
	if s.cluster == nil {
		return true
	}
	leaderID, valid, err := s.cluster.GetLeader(s.clusterID)
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to get leader")
		return false
	}
	return valid && leaderID == s.localNodeID
}

// scanAndExecute scans and executes due tasks.
func (s *Scanner) scanAndExecute() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	currentTime := time.Now().UnixMicro()

	dueTasks, err := s.center.GetDueTasks(ctx, currentTime)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("failed to get due tasks")
		return
	}

	if len(dueTasks) == 0 {
		return
	}

	logger.Logger.Debug().
		Int("count", len(dueTasks)).
		Msg("found due will delay tasks")

	for _, task := range dueTasks {
		s.executeTask(ctx, task)
	}

}

// executeTask 执行单个任务
func (s *Scanner) executeTask(ctx context.Context, task *proto_will_delay.WillDelayTask) {
	if task == nil {
		return
	}
	clientID := task.GetClientID()
	if clientID == "" {
		return
	}
	if !s.willDelayTaskOwnerStillValid(ctx, task) {
		_ = s.deleteTask(ctx, task)
		return
	}
	// 1. 从 Session 读取 WillMessage
	sessionResp, err := s.sessionCenter.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID: clientID,
	})
	if err != nil {
		logger.Logger.Error().
			Err(err).
			Str("client_id", clientID).
			Msg("failed to get session for will message")
		return
	}

	if !sessionResp.Exist || sessionResp.Session == nil {
		logger.Logger.Debug().
			Str("client_id", clientID).
			Msg("session not found, deleting will delay task")
		_ = s.deleteTask(ctx, task)
		return
	}

	willMessage := sessionResp.Session.GetWillMessage()
	if willMessage == nil {
		logger.Logger.Debug().
			Str("client_id", clientID).
			Msg("will message not found in session, deleting task")
		_ = s.deleteTask(ctx, task)
		return
	}

	// 2. 构造并发布 will message
	publish := &packets.Publish{
		Topic:   willMessage.GetTopic(),
		Payload: willMessage.GetPayload(),
		QoS:     byte(willMessage.GetQos()),
		Retain:  willMessage.GetRetain(),
	}

	if props := session.ProtoToWillProperties(willMessage); props != nil {
		publish.Properties = props
	}

	msg := &brokerpublish.Message{
		Publish:      publish,
		SendClientID: clientID,
		OwnerToken:   task.GetOwnerToken(),
		Will:         true,
	}

	// 3. 发送到 will message channel；发布成功后再删除任务，避免临时失败时丢失 will。
	select {
	case s.willMessageChan <- msg:
		if err := s.deleteTask(ctx, task); err != nil {
			logger.Logger.Error().
				Err(err).
				Str("client_id", clientID).
				Msg("failed to delete will delay task")
			return
		}
		logger.Logger.Info().
			Str("client_id", clientID).
			Str("topic", willMessage.GetTopic()).
			Msg("published will message")
	case <-ctx.Done():
		logger.Logger.Warn().
			Str("client_id", clientID).
			Msg("context cancelled while publishing will message")
	}
}

func (s *Scanner) deleteTask(ctx context.Context, task *proto_will_delay.WillDelayTask) error {
	if task == nil {
		return nil
	}
	clientID := task.GetClientID()
	ownerToken := task.GetOwnerToken()
	if ownerToken != "" {
		if center, ok := s.center.(OwnerTaskDeleter); ok {
			return center.DeleteTaskByOwner(ctx, clientID, ownerToken)
		}
	}
	return s.center.DeleteTask(ctx, clientID)
}

func (s *Scanner) willDelayTaskOwnerStillValid(ctx context.Context, task *proto_will_delay.WillDelayTask) bool {
	ownerToken := task.GetOwnerToken()
	if ownerToken == "" || s.sessionCenter == nil {
		return true
	}
	ownerResp, err := s.sessionCenter.GetSessionOwner(ctx, &proto_session.ReadSessionOwnerRequest{
		ClientID: task.GetClientID(),
	})
	if err != nil {
		logger.Logger.Error().
			Err(err).
			Str("client_id", task.GetClientID()).
			Msg("failed to get session owner for will delay fencing")
		return false
	}
	if !ownerResp.GetExist() || ownerResp.GetOwner() == nil {
		return true
	}
	owner := ownerResp.GetOwner()
	if owner.GetOwnerToken() != ownerToken {
		logger.Logger.Warn().
			Str("client_id", task.GetClientID()).
			Str("task_owner_token", ownerToken).
			Str("current_owner_token", owner.GetOwnerToken()).
			Msg("skip stale will delay task")
		return false
	}
	if owner.GetOnline() {
		logger.Logger.Warn().
			Str("client_id", task.GetClientID()).
			Str("owner_token", ownerToken).
			Msg("skip will delay task for online owner")
		return false
	}
	return true
}
