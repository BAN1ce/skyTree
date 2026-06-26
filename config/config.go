package config

// AppConfig is the root runtime configuration for SkyTree.
// It is loaded once at startup and explicitly injected to downstream components.
type AppConfig struct {
	Server   Server         `yaml:"server"`
	Broker   Broker         `yaml:"broker"`
	Storage  Store          `yaml:"storage"`
	Cluster  Cluster        `yaml:"cluster"`
	Console  Console        `yaml:"console"`
	Delivery DeliveryRunner `yaml:"delivery"`
	Logging  Log            `yaml:"logging"`
	Plugins  Plugins        `yaml:"plugins"`
}
