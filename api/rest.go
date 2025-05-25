package api

import (
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Bnei-Baruch/exec-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-cmd/cmd"
	"github.com/spf13/viper"
)

func getFilesList(c *gin.Context) {

	var list []string
	files, err := ioutil.ReadDir(viper.GetString("workflow.capture_path"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
	}

	for _, f := range files {
		list = append(list, f.Name())
	}

	c.JSON(http.StatusOK, list)
}

func getData(c *gin.Context) {
	file := c.Params.ByName("file")

	if _, err := os.Stat(viper.GetString("workflow.capture_path") + file); os.IsNotExist(err) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func getFile(c *gin.Context) {
	file := c.Params.ByName("file")

	http.ServeFile(c.Writer, c.Request, viper.GetString("workflow.capture_path")+file)
}

func isAlive(c *gin.Context) {
	id := c.Params.ByName("id")

	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := utils.PidExists(status.PID)
		if err != nil {
			NewBadRequestError(err).Abort(c)
			return
		}
		if isruning {
			c.JSON(http.StatusOK, gin.H{"result": "success"})
			return
		}
	}

	c.AbortWithStatus(http.StatusNotFound)
}

func startExec(c *gin.Context) {
	ep := c.Params.ByName("ep")

	cfg, err := getConf()
	if err != nil {
		cfg, err = getJson(ep)
		if err != nil {
			NewBadRequestError(err).Abort(c)
			return
		}
	}

	var service string
	var args []string
	var id string
	for _, v := range cfg.Services {
		id = v.ID
		service = v.Name
		args = v.Args
		if len(args) == 0 {
			continue
		}

		if Cmd[id] != nil {
			status := Cmd[id].Status()
			isruning, err := utils.PidExists(status.PID)
			if err != nil {
				continue
			}
			if isruning {
				continue
			}
		}

		if Cmd == nil {
			Cmd = make(map[string]*cmd.Cmd)
		}

		if service == "ffmpeg" {
			cmdOptions := cmd.Options{Buffered: false, Streaming: false}
			os.Setenv("FFREPORT", "file=report_"+id+".log:level=32")
			Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
		} else {
			Cmd[id] = cmd.NewCmd(service, args...)
		}

		Cmd[id].Start()

		time.Sleep(2 * time.Second)

		status := Cmd[id].Status()
		if status.Exit == 1 {
			continue
		}
	}

	// TODO: return exec report
	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func stopExec(c *gin.Context) {

	ep := c.Params.ByName("ep")

	cfg, err := getConf()
	if err != nil {
		cfg, err = getJson(ep)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	var id string
	for _, v := range cfg.Services {
		id = v.ID

		if Cmd[id] == nil {
			continue
		}

		err := Cmd[id].Stop()
		if err != nil {
			continue
		}

		removeProgress("stat_" + id + ".log")
	}

	// TODO: return report
	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func startExecByID(c *gin.Context) {
	id := c.Params.ByName("id")
	ep := c.Params.ByName("ep")

	cfg, err := getConf()
	if err != nil {
		cfg, err = getJson(ep)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"result": "Failed get config"})
			return
		}
	}

	var service string
	var args []string
	for _, v := range cfg.Services {
		if v.ID == id {
			service = v.Name
			args = v.Args
			break
		}
	}

	if len(args) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"result": "Id not found"})
		return
	}

	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := utils.PidExists(status.PID)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		if isruning {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"result": "Already executed"})
			return
		}
	}

	if Cmd == nil {
		Cmd = make(map[string]*cmd.Cmd)
	}

	if service == "ffmpeg" {
		cmdOptions := cmd.Options{Buffered: false, Streaming: false}
		os.Setenv("FFREPORT", "file=report_"+id+".log:level=32")
		Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
	} else {
		Cmd[id] = cmd.NewCmd(service, args...)
	}

	Cmd[id].Start()
	status := Cmd[id].Status()
	if status.Exit == 1 {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"result": "Run Exec Failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func stopExecByID(c *gin.Context) {
	id := c.Params.ByName("id")

	if Cmd[id] == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"result": "Nothing to stop"})
		return
	}

	err := Cmd[id].Stop()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"result": "Stop failed"})
		return
	}

	removeProgress("stat_" + id + ".log")

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func cmdStat(c *gin.Context) {
	id := c.Params.ByName("id")

	if Cmd[id] == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"result": "Nothing to show"})
		return
	}

	status := Cmd[id].Status()

	c.JSON(http.StatusOK, status)

}

func execStatusByID(c *gin.Context) {
	st := make(map[string]interface{})
	id := c.Params.ByName("id")
	ep := c.Params.ByName("ep")

	if Cmd[id] == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"result": "Nothing to show"})
		return
	}

	cfg, err := getConf()
	if err != nil {
		cfg, err = getJson(ep)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"result": "Failed get config"})
			return
		}
	}

	for _, i := range cfg.Services {
		if id == i.ID {
			st["name"] = i.Name
			st["id"] = i.ID
			st["description"] = i.Description
			//st["args"] = i.Args
		}
	}

	if st["name"] == "ffmpeg" {
		progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
		p := <-progress.Start()
		report := strings.Split(p.Stdout[0], "\r")
		n := len(report)
		st["log"] = report[n-1]
	}

	status := Cmd[id].Status()
	isruning, err := utils.PidExists(status.PID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"result": "Failed to get status"})
		return
	}
	st["alive"] = isruning

	//prog := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	//pr := <-prog.Start()
	//for _, line := range pr.Stdout {
	//	args := strings.Split(line, "=")
	//	st[args[0]] = args[1]
	//}

	st["runtime"] = status.Runtime
	st["start"] = status.StartTs
	st["stop"] = status.StopTs

	c.JSON(http.StatusOK, st)
}

func execStatus(c *gin.Context) {
	var id string
	var services []map[string]interface{}
	ep := c.Params.ByName("ep")

	cfg, err := getConf()
	if err != nil {
		cfg, err = getJson(ep)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"result": "Failed get config"})
			return
		}
	}

	for _, i := range cfg.Services {
		st := make(map[string]interface{})
		id = i.ID
		st["name"] = i.Name
		st["id"] = i.ID
		st["description"] = i.Description
		//st["args"] = i.Args

		if Cmd[id] == nil {
			st["runtime"] = 0
			st["start"] = 0
			st["stop"] = 0
			st["alive"] = false
		} else {
			if st["name"] == "ffmpeg" {
				progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
				p := <-progress.Start()
				report := strings.Split(p.Stdout[0], "\r")
				n := len(report)
				st["log"] = report[n-1]
			}

			status := Cmd[id].Status()
			isruning, err := utils.PidExists(status.PID)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"result": "Failed to get status"})
				return
			}
			st["alive"] = isruning

			//prog := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
			//pr := <-prog.Start()
			//for _, line := range pr.Stdout {
			//	args := strings.Split(line, "=")
			//	st[args[0]] = args[1]
			//}

			st["runtime"] = status.Runtime
			st["start"] = status.StartTs
			st["stop"] = status.StopTs
		}

		services = append(services, st)
	}

	c.JSON(http.StatusOK, services)
}

func sysStat(c *gin.Context) {
	temp := cmd.NewCmd("sensors")
	tempStatus := <-temp.Start()

	c.JSON(http.StatusOK, tempStatus.Stdout)
}

func getProgress(c *gin.Context) {
	id := c.Params.ByName("id")

	if Cmd[id] == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"result": "died"})
		removeProgress("stat_" + id + ".log")
		return
	}

	status := Cmd[id].Status()
	isruning, err := utils.PidExists(status.PID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"result": "died"})
		removeProgress("stat_" + id + ".log")
		return
	}

	if isruning {
		progress := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
		p := <-progress.Start()

		ffjson := make(map[string]string)

		for _, line := range p.Stdout {
			args := strings.Split(line, "=")
			ffjson[args[0]] = args[1]
		}

		c.JSON(http.StatusOK, ffjson)
	}
}

func getReport(c *gin.Context) {
	id := c.Params.ByName("id")

	progress := cmd.NewCmd("cat", "report_"+id+".log")
	p := <-progress.Start()

	c.JSON(http.StatusOK, p)
}

func startRemux(c *gin.Context) {
	v := "2"
	id := c.Params.ByName("id")
	ep := c.Params.ByName("ep")
	file := c.Query("file")

	if ep == "fhd" {
		v = "0"
	}

	if ep == "hd" {
		v = "1"
	}

	if ep == "nhd" {
		v = "2"
	}

	args := []string{"-progress", "stat_" + id + ".log", "-hide_banner", "-y", "-i", viper.GetString("workflow.capture_path") + file,
		"-map", "0:v:" + v, "-map", "0:m:language:" + id, "-c", "copy", viper.GetString("workflow.capture_path") + ep + "_" + id + "_" + file}

	if len(args) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"result": "Id not found"})
		return
	}

	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := utils.PidExists(status.PID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"result": "Failed to get status"})
			return
		}
		if isruning {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"result": "Already executed"})
			return
		}
	}

	if Cmd == nil {
		Cmd = make(map[string]*cmd.Cmd)
	}
	Cmd[id] = cmd.NewCmd("ffmpeg", args...)

	Cmd[id].Start()
	status := Cmd[id].Status()
	if status.Exit == 1 {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"result": "Run Exec Failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func getUploadPath(ep string) string {

	switch ep {
	case "insert":
		return "/backup/tmp/insert/"
	case "jobs":
		return "/backup/jobs/"
	case "products":
		return "/backup/files/upload/"
	case "aricha":
		return "/backup/aricha/"
	case "aklada":
		return "/backup/tmp/akladot/"
	case "gibuy":
		return "/backup/tmp/gibuy/"
	case "carbon":
		return "/backup/tmp/carbon/"
	case "dgima":
		return "/backup/dgima/"
	case "proxy":
		return "/backup/tmp/proxy/"
	case "youtube":
		return "/backup/tmp/youtube/"
	case "coder":
		return "/backup/tmp/coder/"
	case "muxer":
		return "/backup/tmp/muxer/"
	default:
		return "/backup/tmp/upload/"
	}
}

func HandleUploadGin(c *gin.Context) {
	endpoint := c.Param("ep")
	uploadPath := getUploadPath(endpoint)

	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		os.MkdirAll(uploadPath, 0755)
	}

	var (
		n     int
		err   error
		mr    *multipart.Reader
		part  *multipart.Part
		chunk = make([]byte, 10485760)
		u     Upload
	)

	if mr, err = c.Request.MultipartReader(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for {
		var tempfile *os.File
		var filesize int
		var uploaded bool

		if part, err = mr.NextPart(); err != nil {
			if err != io.EOF {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				if tempfile != nil {
					os.Remove(tempfile.Name())
				}
			} else {
				c.JSON(http.StatusOK, u)
			}
			return
		}

		u.Filename = part.FileName()
		u.Mimetype = part.Header.Get("Content-Type")
		u.Url = filepath.Join(uploadPath, u.Filename)

		tempfile, err = os.CreateTemp(uploadPath, part.FileName()+".*")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer tempfile.Close()

		for !uploaded {
			if n, err = part.Read(chunk); err != nil {
				if err != io.EOF {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					os.Remove(tempfile.Name())
					return
				}
				uploaded = true
			}

			if n, err = tempfile.Write(chunk[:n]); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				os.Remove(tempfile.Name())
				return
			}
			filesize += n
		}

		os.Rename(tempfile.Name(), u.Url)
		u.UploadProps(u.Url, endpoint) // as in original
	}
}

// Configuration management handlers

func getConfig(c *gin.Context) {
	config, err := getConf()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	c.JSON(http.StatusOK, config)
}

func updateServiceArgs(c *gin.Context) {
	id := c.Params.ByName("id")

	var requestBody struct {
		Args []string `json:"args" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	config, err := getConf()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	service := findServiceByID(config, id)
	if service == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	service.Args = requestBody.Args

	err = saveConf(config)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success", "service": service})
}

func addService(c *gin.Context) {
	var newService Service

	if err := c.ShouldBindJSON(&newService); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	if newService.ID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Service ID is required"})
		return
	}

	config, err := getConf()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	// Check if service with this ID already exists
	if findServiceByID(config, newService.ID) != nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "Service with this ID already exists"})
		return
	}

	config.Services = append(config.Services, newService)

	err = saveConf(config)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"result": "success", "service": newService})
}

func deleteService(c *gin.Context) {
	id := c.Params.ByName("id")

	config, err := getConf()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	if !removeServiceByID(config, id) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	err = saveConf(config)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success", "message": "Service deleted"})
}
