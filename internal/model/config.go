package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Account 账户信息（加密存储）
type Account struct {
	UserName      string `json:"userName"`
	Password      string `json:"passWord"`
	Authorization string `json:"authorization"`
	DeviceType    string `json:"deviceType"`
	OSVersion     string `json:"osVersion"`
	LoginUUID     string `json:"loginuuid"`
}

// Config 全局配置
type Config struct {
	CurrentAccount string             `json:"currentAccount"`
	Accounts       map[string]Account `json:"accounts"`
	Settings       map[string]any     `json:"settings"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configPath   string
	configLock   sync.RWMutex
)

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "123pan-cli")
}

func getConfigPath() string {
	if configPath != "" {
		return configPath
	}
	dir := configDir()
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "config.json")
}

func LoadConfig() *Config {
	configOnce.Do(func() {
		globalConfig = &Config{
			Accounts: make(map[string]Account),
			Settings: make(map[string]any),
		}
		data, err := os.ReadFile(getConfigPath())
		if err != nil {
			return
		}
		json.Unmarshal(data, globalConfig)
		if globalConfig.Accounts == nil {
			globalConfig.Accounts = make(map[string]Account)
		}
		if globalConfig.Settings == nil {
			globalConfig.Settings = make(map[string]any)
		}
	})
	return globalConfig
}

func SaveConfig() error {
	configLock.Lock()
	defer configLock.Unlock()
	cfg := LoadConfig()
	return writeConfigLocked(cfg)
}

// writeConfigLocked 写入配置（调用方必须持有 configLock 写锁）
func writeConfigLocked(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0600)
}

func GetAccount(userName string) *Account {
	cfg := LoadConfig()
	configLock.RLock()
	defer configLock.RUnlock()
	if userName == "" {
		userName = cfg.CurrentAccount
	}
	if acc, ok := cfg.Accounts[userName]; ok {
		return &acc
	}
	return nil
}

// ListAccounts 返回所有已保存的账号列表
func ListAccounts() []Account {
	cfg := LoadConfig()
	configLock.RLock()
	defer configLock.RUnlock()
	var accounts []Account
	for _, acc := range cfg.Accounts {
		accounts = append(accounts, acc)
	}
	return accounts
}

func SaveAccount(acc Account) error {
	cfg := LoadConfig()
	configLock.Lock()
	defer configLock.Unlock()
	cfg.Accounts[acc.UserName] = acc
	cfg.CurrentAccount = acc.UserName
	return writeConfigLocked(cfg)
}

func GetSetting(key string, defaultVal any) any {
	cfg := LoadConfig()
	configLock.RLock()
	defer configLock.RUnlock()
	if v, ok := cfg.Settings[key]; ok {
		return v
	}
	return defaultVal
}

func GetSettingInt(key string, defaultVal int) int {
	v := GetSetting(key, nil)
	if v == nil {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	default:
		return defaultVal
	}
}

func SetSetting(key string, value any) error {
	cfg := LoadConfig()
	configLock.Lock()
	defer configLock.Unlock()
	cfg.Settings[key] = value
	return writeConfigLocked(cfg)
}
