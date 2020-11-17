package api

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

type Config struct {
	Services []Service
}

type Service struct {
	ID   string
	Name string
	Args []string
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

func PidExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid pid %v", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false, err
	}
	switch errno {
	case syscall.ESRCH:
		return false, nil
	case syscall.EPERM:
		return true, nil
	}
	return false, err
}

func getConf() (*Config, error) {
	file, err := os.Open(os.Getenv("WORK_DIR") + "conf.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	Config := Config{}
	err = decoder.Decode(&Config)
	fmt.Println(Config)
	if err != nil {
		return nil, err
	}
	return &Config, nil
}
