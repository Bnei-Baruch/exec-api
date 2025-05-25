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
	Name        string
	Ip          string
	Description string
	Services    []Service
}

type Service struct {
	ID          string
	Name        string
	Description string
	Args        []string
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
