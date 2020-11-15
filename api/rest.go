package api

import (
	"errors"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/pkg/httputil"
	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
	"github.com/go-cmd/cmd"
	"net/http"
)

type Proc struct {
	Stat *cmd.Status
	Cmd  *cmd.Cmd
}

func checkRole(role string, r *http.Request) bool {
	if rCtx, ok := middleware.ContextFromRequest(r); ok {
		if rCtx.IDClaims != nil {
			for _, r := range rCtx.IDClaims.RealmAccess.Roles {
				if r == role {
					return true
				}
			}
		}
	}
	return false
}

func (a *App) getData(w http.ResponseWriter, r *http.Request) {

	// Check role
	authRoot := checkRole("auth_root", r)
	if !authRoot {
		e := errors.New("bad permission")
		httputil.NewUnauthorizedError(e).Abort(w, r)
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) stopData(w http.ResponseWriter, r *http.Request) {

	err := a.Cmd.Stop()
	if err != nil {
		httputil.NewUnauthorizedError(err).Abort(w, r)
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) statData(w http.ResponseWriter, r *http.Request) {

	fmt.Println(a.Cmd.Status())

	httputil.RespondSuccess(w)
}

func (a *App) execData(w http.ResponseWriter, r *http.Request) {

	// Start a long-running process, capture stdout and stderr
	a.Cmd = cmd.NewCmd("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc", "-pix_fmt", "yuv420p", "-f", "mp4", "/dev/null")

	a.Cmd.Start()

	httputil.RespondSuccess(w)
}
