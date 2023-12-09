package api

import "github.com/gin-gonic/gin"

func SetupRoutes(router *gin.Engine) {
	router.GET("/:ep/sysstat", sysStat)
	router.GET("/:ep/status", execStatus)
	router.GET("/:ep/start", startExec)
	router.GET("/:ep/stop", stopExec)
	router.GET("/:ep/start/:id", startExecByID)
	router.GET("/:ep/stop/:id", stopExecByID)
	router.GET("/:ep/status/:id", execStatusByID)
	router.GET("/:ep/cmdstat/:id", cmdStat)
	router.GET("/:ep/progress/:id", getProgress)
	router.GET("/:ep/report/:id", getReport)
	router.GET("/:ep/alive/:id", isAlive)
	router.GET("/:ep/remux/:id", startRemux)

	router.GET("/test/:file", getData)
	router.GET("/get/:file", getFile)
	router.GET("/files/list", getFilesList)
}
