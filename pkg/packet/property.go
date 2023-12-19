package packet

//type User struct {
//	Key   string `json:"key"`
//	Value string `json:"value"`
//}
//type WillProperties struct {
//	DelayInterval   uint32 `json:"delay_interval,omitempty"`
//	PayloadFormat   int    `json:"payload_format"`
//	ExpiryInterval  uint32 `json:"expiry_interval"`
//	ContentType     string `json:"content_type"`
//	ResponseTopic   string `json:"response_topic"`
//	CorrelationData []byte `json:"correlation_data,omitempty"`
//	UserProperties  []User `json:"user_properties,omitempty"`
//}
//
//func PropertyToWillProperties(properties *packets.Properties) *WillProperties {
//	var (
//		will = &WillProperties{}
//	)
//	if properties.WillDelayInterval != nil {
//		will.DelayInterval = int64(*properties.WillDelayInterval)
//	}
//	if properties.PayloadFormat != nil {
//		will.PayloadFormat = int64(*properties.PayloadFormat)
//	}
//
//	if properties.MessageExpiry != nil {
//		will.ExpiryInterval = int64(*properties.MessageExpiry)
//	}
//	will.ContentType = properties.ContentType
//	will.ResponseTopic = properties.ResponseTopic
//	will.CorrelationData = properties.CorrelationData
//	for _, v := range properties.User {
//		will.UserProperties = append(will.UserProperties, User{
//			Key:   v.Key,
//			Value: v.Value,
//		})
//	}
//	return will
//}
//
//type ConnectProperties struct {
//	SessionExpiryInterval int64  `json:"session_expiry_interval,omitempty"`
//	ReceiveMaximum        int64  `json:"receive_maximum,omitempty"`
//	MaximumPacketSize     int64  `json:"maximum_packet_size,omitempty"`
//	TopicAliasMaximum     int64  `json:"topic_alias_maximum,omitempty"`
//	RequestResponseInfo   string `json:"request_response_info,omitempty"`
//	RequestProblemInfo    bool   `json:"request_problem_info,omitempty"`
//	UserProperties        []User `json:"user_properties,omitempty"`
//	AuthMethod            string `json:"auth_method,omitempty"`
//	AuthData              []byte `json:"auth_data,omitempty"`
//}
//
//func PropertyToConnectProperties(properties *packets.Properties) (*ConnectProperties, error) {
//	var (
//		connect = &ConnectProperties{}
//	)
//	if properties.SessionExpiryInterval != nil {
//		connect.SessionExpiryInterval = int64(*properties.SessionExpiryInterval)
//	}
//	if properties.ReceiveMaximum != nil {
//		connect.ReceiveMaximum = int64(*properties.ReceiveMaximum)
//	}
//	if properties.MaximumPacketSize != nil {
//		connect.MaximumPacketSize = int64(*properties.MaximumPacketSize)
//	}
//	if properties.TopicAliasMaximum != nil {
//		connect.TopicAliasMaximum = int64(*properties.TopicAliasMaximum)
//	}
//	if properties.RequestResponseInfo != nil {
//		if !((*properties.RequestResponseInfo) == 0x01 || (*properties.RequestResponseInfo) == 0x00) {
//			return nil, errs.ErrInvalidRequestResponseInfo
//		}
//		connect.RequestResponseInfo = properties.ResponseInfo
//	}
//	if properties.RequestProblemInfo != nil {
//		if !((*properties.RequestProblemInfo) == 0x01 || (*properties.RequestProblemInfo) == 0x00) {
//			return nil, errs.ErrInvalidRequestProblemInfo
//		}
//		connect.RequestProblemInfo = (*properties.RequestProblemInfo) == 0x01
//	}
//	connect.AuthMethod = properties.AuthMethod
//	connect.AuthData = properties.AuthData
//	for _, v := range properties.User {
//		connect.UserProperties = append(connect.UserProperties, User{
//			Key:   v.Key,
//			Value: v.Value,
//		})
//	}
//	return connect, nil
//}
