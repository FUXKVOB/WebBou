package webbou

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	QUICAddr     string
	TCPAddr      string
	MaxStreams   int
	MaxFrameSize int
	TLSConfig    interface{}
}

func (c *ServerConfig) Validate() error {
	if c.QUICAddr == "" {
		c.QUICAddr = "0.0.0.0:8443"
	}
	if c.TCPAddr == "" {
		c.TCPAddr = "0.0.0.0:8444"
	}
	if c.MaxStreams == 0 {
		c.MaxStreams = 100
	}
	if c.MaxFrameSize == 0 {
		c.MaxFrameSize = 65536
	}
	return nil
}

type YAMLConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	QUIC struct {
		Enabled bool   `yaml:"enabled"`
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
	} `yaml:"quic"`
	TCP struct {
		Enabled bool   `yaml:"enabled"`
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
	} `yaml:"tcp"`
	Limits struct {
		MaxConnections int `yaml:"max_connections"`
		MaxFrameSize   int `yaml:"max_frame_size"`
		RateLimitPerIP int `yaml:"rate_limit_per_ip"`
	} `yaml:"limits"`
	Keepalive struct {
		Interval int `yaml:"interval"`
		Timeout  int `yaml:"timeout"`
	} `yaml:"keepalive"`
	Compress struct {
		Enabled bool `yaml:"enabled"`
		Level   int  `yaml:"level"`
	} `yaml:"compress"`
	Crypto struct {
		Enabled    bool   `yaml:"enabled"`
		Algorithm string `yaml:"algorithm"`
	} `yaml:"crypto"`
	Metrics struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	} `yaml:"metrics"`
	Logging struct {
		Level string `yaml:"level"`
		JSON  bool   `yaml:"json"`
	} `yaml:"logging"`
	Health struct {
		Enabled       bool   `yaml:"enabled"`
		Port         int    `yaml:"port"`
		ReadinessPath string `yaml:"readiness_path"`
		LivenessPath string `yaml:"liveness_path"`
	} `yaml:"health"`
	Debug struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	} `yaml:"debug"`
}

func LoadServerConfigFromYAML(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &ServerConfig{}
	cfg.QUICAddr = fmt.Sprintf("%s:%d", yamlCfg.QUIC.Host, yamlCfg.QUIC.Port)
	if cfg.QUICAddr == ":" {
		cfg.QUICAddr = "0.0.0.0:8443"
	}

	cfg.TCPAddr = fmt.Sprintf("%s:%d", yamlCfg.TCP.Host, yamlCfg.TCP.Port)
	if cfg.TCPAddr == ":" {
		cfg.TCPAddr = "0.0.0.0:8444"
	}

	cfg.MaxFrameSize = yamlCfg.Limits.MaxFrameSize
	if cfg.MaxFrameSize == 0 {
		cfg.MaxFrameSize = 65536
	}

	return cfg, nil
}

type ConfigManager struct {
	mu       sync.RWMutex
	config   *ServerConfig
	watcher  *fsWatcher
	version  int
	onChange func(*ServerConfig) error
}

type fsWatcher struct {
	path    string
	lastMod time.Time
	ticker  *time.Ticker
}

func NewConfigManagerFromYAML(cfg *ServerConfig) *ConfigManager {
	return &ConfigManager{
		config:  cfg,
		version: 1,
	}
}

func (cm *ConfigManager) Get() *ServerConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

func (cm *ConfigManager) Update(newCfg *ServerConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if newCfg != nil {
		cm.config = newCfg
		cm.version++
	}

	if cm.onChange != nil {
		return cm.onChange(cm.config)
	}

	return nil
}

func (cm *ConfigManager) Watch(path string, interval time.Duration) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	interval2 := interval
	if interval2 == 0 {
		interval2 = time.Second
	}

	cm.watcher = &fsWatcher{
		path:   path,
		ticker: time.NewTicker(interval2),
	}

	go func() {
		for range cm.watcher.ticker.C {
			cm.reloadIfChanged()
		}
	}()

	return nil
}

func (cm *ConfigManager) reloadIfChanged() {
	info, err := os.Stat(cm.watcher.path)
	if err != nil {
		return
	}

	modTime := info.ModTime()
	if modTime.After(cm.watcher.lastMod) {
		cm.watcher.lastMod = modTime

		if newCfg, err := LoadServerConfigFromYAML(cm.watcher.path); err == nil {
			cm.Update(newCfg)
		}
	}
}

func (cm *ConfigManager) OnChange(fn func(*ServerConfig) error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onChange = fn
}

func (cm *ConfigManager) Version() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.version
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		QUICAddr:     "0.0.0.0:8443",
		TCPAddr:      "0.0.0.0:8444",
		MaxStreams:   100,
		MaxFrameSize: 65536,
	}
}