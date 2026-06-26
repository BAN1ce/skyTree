package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

// HTTPProvider HTTP认证提供者
type HTTPProvider struct {
	endpoint string
	timeout  time.Duration
	client   *http.Client
}

// NewHTTPProvider 创建HTTP认证提供者
func NewHTTPProvider(endpoint string, timeoutSeconds int) *HTTPProvider {
	validTimeout := ValidateTimeout(timeoutSeconds)
	timeout := time.Duration(validTimeout) * time.Second

	return &HTTPProvider{
		endpoint: endpoint,
		timeout:  timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Authenticate 执行HTTP认证请求
func (p *HTTPProvider) Authenticate(ctx context.Context, clientID string, authPacket *packets.Auth) (*packets.Auth, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		metric.RecordAuthDuration("http", duration)
	}()

	// 创建带超时的context，最大10秒
	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	jsonData, err := json.Marshal(authRequestBody(clientID, authPacket))
	if err != nil {
		metric.RecordAuthRequestFailed("http", "marshal_error")
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("failed to marshal auth request")
		return nil, fmt.Errorf("failed to marshal auth request: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, p.endpoint, bytes.NewReader(jsonData))
	if err != nil {
		metric.RecordAuthRequestFailed("http", "request_error")
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("failed to create HTTP request")
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	body, statusCode, err := p.doAuthRequest(reqCtx, req, clientID)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		metric.RecordAuthRequestFailed("http", fmt.Sprintf("http_%d", statusCode))
		logger.Logger.Warn().Int("status", statusCode).Str("client", clientID).
			Msg("AUTH HTTP request returned non-200 status")
		return &packets.Auth{ReasonCode: packets.AuthReauthenticate}, nil
	}

	authResp, err := parseAuthResponseBody(body)
	if err != nil {
		metric.RecordAuthRequestFailed("http", "unmarshal_error")
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("failed to unmarshal HTTP response")
		return nil, fmt.Errorf("failed to unmarshal HTTP response: %w", err)
	}
	recordHTTPAuthResult(authResp)
	return authResp, nil
}

func authRequestBody(clientID string, authPacket *packets.Auth) map[string]interface{} {
	reqBody := map[string]interface{}{
		"client_id":   clientID,
		"reason_code": authPacket.ReasonCode,
	}
	if authPacket.Properties == nil {
		return reqBody
	}
	reqBody["auth_method"] = authPacket.Properties.AuthMethod
	if len(authPacket.Properties.AuthData) > 0 {
		reqBody["auth_data"] = authPacket.Properties.AuthData
	}
	if len(authPacket.Properties.User) > 0 {
		reqBody["user_properties"] = authUserPropertiesBody(authPacket.Properties.User)
	}
	if authPacket.Properties.ReasonString != "" {
		reqBody["reason_string"] = authPacket.Properties.ReasonString
	}
	return reqBody
}

func authUserPropertiesBody(users []packets.User) []map[string]string {
	userProps := make([]map[string]string, 0, len(users))
	for _, u := range users {
		userProps = append(userProps, map[string]string{
			"key":   u.Key,
			"value": u.Value,
		})
	}
	return userProps
}

func (p *HTTPProvider) doAuthRequest(reqCtx context.Context, req *http.Request, clientID string) ([]byte, int, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, p.handleAuthRequestError(reqCtx, clientID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		metric.RecordAuthRequestFailed("http", "read_error")
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("failed to read HTTP response")
		return nil, resp.StatusCode, fmt.Errorf("failed to read HTTP response: %w", err)
	}
	return body, resp.StatusCode, nil
}

func (p *HTTPProvider) handleAuthRequestError(reqCtx context.Context, clientID string, err error) error {
	errorType := "network_error"
	if reqCtx.Err() == context.DeadlineExceeded {
		errorType = "timeout"
		logger.Logger.Warn().Err(err).Str("client", clientID).Dur("timeout", p.timeout).
			Msg("AUTH HTTP request timeout")
	} else {
		logger.Logger.Error().Err(err).Str("client", clientID).Msg("AUTH HTTP request failed")
	}
	metric.RecordAuthRequestFailed("http", errorType)
	return fmt.Errorf("HTTP request failed: %w", err)
}

func parseAuthResponseBody(body []byte) (*packets.Auth, error) {
	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, err
	}
	authResp := &packets.Auth{ReasonCode: packets.AuthSuccess}
	if rc, ok := respData["reason_code"].(float64); ok {
		authResp.ReasonCode = byte(rc)
	}
	if props, ok := respData["properties"].(map[string]interface{}); ok {
		authResp.Properties = authPropertiesFromHTTP(props)
	}
	return authResp, nil
}

func authPropertiesFromHTTP(props map[string]interface{}) *packets.AuthProperties {
	authProps := &packets.AuthProperties{}
	if authMethod, ok := props["auth_method"].(string); ok {
		authProps.AuthMethod = authMethod
	}
	if authData, ok := props["auth_data"].(string); ok {
		authProps.AuthData = []byte(authData)
	} else if authDataBytes, ok := props["auth_data"].([]byte); ok {
		authProps.AuthData = authDataBytes
	}
	if reasonString, ok := props["reason_string"].(string); ok {
		authProps.ReasonString = reasonString
	}
	if userProps, ok := props["user_properties"].([]interface{}); ok {
		authProps.User = authUsersFromHTTP(userProps)
	}
	return authProps
}

func authUsersFromHTTP(userProps []interface{}) []packets.User {
	users := make([]packets.User, 0, len(userProps))
	for _, up := range userProps {
		upMap, ok := up.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := upMap["key"].(string)
		value, _ := upMap["value"].(string)
		if key != "" {
			users = append(users, packets.User{Key: key, Value: value})
		}
	}
	return users
}

func recordHTTPAuthResult(authResp *packets.Auth) {
	if authResp.ReasonCode == packets.AuthSuccess {
		metric.RecordAuthRequest("http", "success")
		return
	}
	metric.RecordAuthRequest("http", "failed")
}
