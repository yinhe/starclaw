package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig          `yaml:"server"`
	Repos  map[string]RepoConfig `yaml:"repos"`
}

type ServerConfig struct {
	Port     string `yaml:"port"`
	Secret   string `yaml:"secret"`
	ReposDir string `yaml:"repos_dir"`
	SSHUser  string `yaml:"ssh_user"`
}

type RepoConfig struct {
	Description string         `yaml:"description"`
	Targets     []TargetConfig `yaml:"targets"`
}

type TargetConfig struct {
	Name       string `yaml:"name"`
	WormURL    string `yaml:"worm_url"`
	DeployPath string `yaml:"deploy_path"`
	DeployCmd  string `yaml:"deploy_cmd"`
	Subdir     string `yaml:"subdir"`
	Branch     string `yaml:"branch"`
}

type WormConfig struct {
	Port       string            `yaml:"port"`
	Secret     string            `yaml:"secret"`
	DeployDirs map[string]string `yaml:"deploy_dirs"`
}

var C Config
var W WormConfig

func LoadServer(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("[nydus] failed to read config %s: %v", path, err)
	}
	if err := yaml.Unmarshal(data, &C); err != nil {
		log.Fatalf("[nydus] failed to parse config: %v", err)
	}
	if C.Server.Port == "" {
		C.Server.Port = "8095"
	}
	if C.Server.ReposDir == "" {
		C.Server.ReposDir = "/data/nydus/repos"
	}
	if C.Server.SSHUser == "" {
		C.Server.SSHUser = "git"
	}
	log.Printf("[nydus] loaded config: %d repos, repos_dir=%s", len(C.Repos), C.Server.ReposDir)
}

func LoadWorm(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("[worm] failed to read config %s: %v", path, err)
	}
	if err := yaml.Unmarshal(data, &W); err != nil {
		log.Fatalf("[worm] failed to parse config: %v", err)
	}
	if W.Port == "" {
		W.Port = "8096"
	}
	log.Printf("[worm] loaded config: port=%s, %d deploy dirs", W.Port, len(W.DeployDirs))
}
