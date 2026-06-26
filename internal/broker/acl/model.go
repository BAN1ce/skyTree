package acl

// Action represents an operation to authorize.
type Action string

const (
	ActionPublish   Action = "pub"
	ActionSubscribe Action = "sub"
)

// Identity is the subject of ACL evaluation.
// For now we support exact match by Username and/or ClientID.
type Identity struct {
	Username string `yaml:"username" json:"username"`
	ClientID string `yaml:"client_id" json:"client_id"`
}

// RuleEntry holds allow/deny filters for actions.
type RuleEntry struct {
	Pub []string `yaml:"pub" json:"pub"`
	Sub []string `yaml:"sub" json:"sub"`
}

// Rule defines ACL policy for a given identity.
// Deny has higher priority than Allow.
type Rule struct {
	Identity Identity  `yaml:"identity" json:"identity"`
	Allow    RuleEntry `yaml:"allow" json:"allow"`
	Deny     RuleEntry `yaml:"deny" json:"deny"`
}

// Ruleset is a full ACL configuration.
type Ruleset struct {
	DefaultDeny bool   `yaml:"default_deny" json:"default_deny"`
	Rules       []Rule `yaml:"rules" json:"rules"`
}
