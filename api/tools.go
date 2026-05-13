package api

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Name            string
	Ip              string
	Description     string
	Services        []Service
	AllowedCommands []AllowedCommand `json:"allowed_commands,omitempty"`
}

type Service struct {
	ID          string
	Name        string
	Description string
	Args        []string
}

// AllowedCommand describes a single shell command that is permitted to be
// invoked through the generic command runner (MQTT and HTTP). Commands are
// looked up by ID and executed exactly as defined here — no extra arguments
// are accepted from the caller.
type AllowedCommand struct {
	ID             string   `json:"id"`
	Cmd            string   `json:"cmd"`
	Args           []string `json:"args,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
	Description    string   `json:"description,omitempty"`
}

// Timeout bounds applied when AllowedCommand.TimeoutSeconds is read.
const (
	DefaultCommandTimeoutSeconds = 10
	MinCommandTimeoutSeconds     = 1
	MaxCommandTimeoutSeconds     = 300
)

// normalizeTimeout clamps timeout_seconds into the allowed range and applies
// the default when the value is zero (i.e. not specified in the config).
func normalizeTimeout(seconds int) int {
	if seconds <= 0 {
		return DefaultCommandTimeoutSeconds
	}
	if seconds < MinCommandTimeoutSeconds {
		return MinCommandTimeoutSeconds
	}
	if seconds > MaxCommandTimeoutSeconds {
		return MaxCommandTimeoutSeconds
	}
	return seconds
}

// normalizeAllowedCommands clamps TimeoutSeconds for every entry. It mutates
// the slice in place so the rest of the code can rely on sane values.
func normalizeAllowedCommands(cmds []AllowedCommand) {
	for i := range cmds {
		cmds[i].TimeoutSeconds = normalizeTimeout(cmds[i].TimeoutSeconds)
	}
}

// FindAllowedCommandByID returns a pointer to the matching AllowedCommand from
// cfg.AllowedCommands or nil when no entry has the requested id.
func FindAllowedCommandByID(cfg *Config, id string) *AllowedCommand {
	if cfg == nil {
		return nil
	}
	for i := range cfg.AllowedCommands {
		if cfg.AllowedCommands[i].ID == id {
			return &cfg.AllowedCommands[i]
		}
	}
	return nil
}

type Upload struct {
	Filename  string `json:"file_name"`
	Extension string `json:"extension,omitempty"`
	Sha1      string `json:"sha1"`
	Size      int64  `json:"size"`
	Mimetype  string `json:"type"`
	Url       string `json:"url"`
}

func getJson(ep string) (*Config, error) {
	req, err := http.NewRequest("GET", viper.GetString("server.cfg_url")+ep, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	conf := Config{}
	err = json.Unmarshal(body, &conf)
	if err != nil {
		return nil, err
	}

	normalizeAllowedCommands(conf.AllowedCommands)
	return &conf, nil
}

func logTail(fname string) {
	file, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := make([]byte, 215)
	stat, err := os.Stat(fname)
	start := stat.Size() - 215
	_, err = file.ReadAt(buf, start)
	if err == nil {
		fmt.Printf("%s\n", buf)
	}

}

func getConf() (*Config, error) {
	file, err := os.Open("conf.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	Config := Config{}
	err = decoder.Decode(&Config)
	//fmt.Println(Config)
	if err != nil {
		return nil, err
	}
	normalizeAllowedCommands(Config.AllowedCommands)
	return &Config, nil
}

func removeProgress(file string) {
	_, err := os.Stat(file)
	if err == nil {
		e := os.Remove(file)
		if e != nil {
			fmt.Printf("%s\n", e)
		}
	}
}

func saveConf(config *Config) error {
	file, err := os.Create("conf.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(config)
	if err != nil {
		return err
	}
	return nil
}

func findServiceByID(config *Config, id string) *Service {
	for i := range config.Services {
		if config.Services[i].ID == id {
			return &config.Services[i]
		}
	}
	return nil
}

func removeServiceByID(config *Config, id string) bool {
	for i, service := range config.Services {
		if service.ID == id {
			config.Services = append(config.Services[:i], config.Services[i+1:]...)
			return true
		}
	}
	return false
}

func (u *Upload) UploadProps(filepath string, ep string) error {

	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	u.Size = fi.Size()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	u.Sha1 = hex.EncodeToString(h.Sum(nil))

	return nil
}
