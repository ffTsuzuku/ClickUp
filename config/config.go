package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type Profile struct {
	ClickupAPIKey   string `json:"clickup_api_key"`
	ClickupUserName string `json:"clickup_user_name"`
	ClickupTeamID   string `json:"clickup_team_id"`
	ClickupSpaceID  string `json:"clickup_space_id"`
	ClickupFolderID string `json:"clickup_folder_id"`
	ClickupListID   string `json:"clickup_list_id"`
}

type Config struct {
	ActiveProfile string              `json:"active_profile,omitempty"`
	Profiles      map[string]*Profile `json:"profiles,omitempty"`

	// Legacy flat fields are kept for backward compatibility and for the
	// active profile view used throughout the app.
	ClickupAPIKey   string `json:"clickup_api_key,omitempty"`
	ClickupUserName string `json:"clickup_user_name,omitempty"`
	ClickupTeamID   string `json:"clickup_team_id,omitempty"`
	ClickupSpaceID  string `json:"clickup_space_id,omitempty"`
	ClickupFolderID string `json:"clickup_folder_id,omitempty"`
	ClickupListID   string `json:"clickup_list_id,omitempty"`
}

func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "totui", "totui.json"), nil
}

func configDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "totui"), nil
}

func (c *Config) activeProfileData() *Profile {
	if c.Profiles == nil || c.ActiveProfile == "" {
		return nil
	}
	return c.Profiles[c.ActiveProfile]
}

func (c *Config) syncFromActiveProfile() {
	p := c.activeProfileData()
	if p == nil {
		return
	}
	c.ClickupAPIKey = p.ClickupAPIKey
	c.ClickupUserName = p.ClickupUserName
	c.ClickupTeamID = p.ClickupTeamID
	c.ClickupSpaceID = p.ClickupSpaceID
	c.ClickupFolderID = p.ClickupFolderID
	c.ClickupListID = p.ClickupListID
}

func (c *Config) syncToActiveProfile() {
	if c.Profiles == nil {
		c.Profiles = make(map[string]*Profile)
	}
	if c.ActiveProfile == "" {
		c.ActiveProfile = "default"
	}
	c.Profiles[c.ActiveProfile] = &Profile{
		ClickupAPIKey:   c.ClickupAPIKey,
		ClickupUserName: c.ClickupUserName,
		ClickupTeamID:   c.ClickupTeamID,
		ClickupSpaceID:  c.ClickupSpaceID,
		ClickupFolderID: c.ClickupFolderID,
		ClickupListID:   c.ClickupListID,
	}
}

func (c *Config) normalizeProfiles() {
	if c.Profiles == nil {
		c.Profiles = make(map[string]*Profile)
	}

	if len(c.Profiles) == 0 {
		c.Profiles["default"] = &Profile{
			ClickupAPIKey:   c.ClickupAPIKey,
			ClickupUserName: c.ClickupUserName,
			ClickupTeamID:   c.ClickupTeamID,
			ClickupSpaceID:  c.ClickupSpaceID,
			ClickupFolderID: c.ClickupFolderID,
			ClickupListID:   c.ClickupListID,
		}
	}

	if c.ActiveProfile == "" {
		if _, ok := c.Profiles["default"]; ok {
			c.ActiveProfile = "default"
		} else {
			names := c.ProfileNames()
			if len(names) > 0 {
				c.ActiveProfile = names[0]
			}
		}
	}

	if _, ok := c.Profiles[c.ActiveProfile]; !ok {
		names := c.ProfileNames()
		if len(names) == 0 {
			c.Profiles["default"] = &Profile{}
			c.ActiveProfile = "default"
		} else {
			c.ActiveProfile = names[0]
		}
	}
}

func (c *Config) ensureProfiles() {
	c.normalizeProfiles()
	c.syncFromActiveProfile()
}

func (c *Config) ActiveProfileName() string {
	c.ensureProfiles()
	return c.ActiveProfile
}

func (c *Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) HasProfile(name string) bool {
	c.ensureProfiles()
	_, ok := c.Profiles[name]
	return ok
}

func (c *Config) SetActiveProfile(name string) bool {
	c.ensureProfiles()
	if _, ok := c.Profiles[name]; !ok {
		return false
	}
	c.ActiveProfile = name
	c.syncFromActiveProfile()
	return true
}

func (c *Config) SaveCurrentAsProfile(name string) {
	c.ensureProfiles()
	c.Profiles[name] = &Profile{
		ClickupAPIKey:   c.ClickupAPIKey,
		ClickupUserName: c.ClickupUserName,
		ClickupTeamID:   c.ClickupTeamID,
		ClickupSpaceID:  c.ClickupSpaceID,
		ClickupFolderID: c.ClickupFolderID,
		ClickupListID:   c.ClickupListID,
	}
	c.ActiveProfile = name
	c.syncFromActiveProfile()
}

func (c *Config) CreateProfile(name string) bool {
	c.ensureProfiles()
	if name == "" {
		return false
	}
	if _, ok := c.Profiles[name]; ok {
		return false
	}
	c.Profiles[name] = &Profile{}
	c.ActiveProfile = name
	c.syncFromActiveProfile()
	return true
}

func (c *Config) DeleteProfile(name string) (string, bool) {
	c.ensureProfiles()
	if _, ok := c.Profiles[name]; !ok {
		return "", false
	}

	delete(c.Profiles, name)

	if len(c.Profiles) == 0 {
		c.Profiles["default"] = &Profile{}
		c.ActiveProfile = "default"
		c.syncFromActiveProfile()
		return c.ActiveProfile, true
	}

	if c.ActiveProfile == name {
		if _, ok := c.Profiles["default"]; ok {
			c.ActiveProfile = "default"
		} else {
			names := c.ProfileNames()
			c.ActiveProfile = names[0]
		}
	}

	c.syncFromActiveProfile()
	return c.ActiveProfile, true
}

func (c *Config) ClearRoutingDefaults() {
	c.ClickupTeamID = ""
	c.ClickupSpaceID = ""
	c.ClickupFolderID = ""
	c.ClickupListID = ""
}

func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.ensureProfiles()
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	cfg.normalizeProfiles()
	cfg.syncToActiveProfile()

	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
