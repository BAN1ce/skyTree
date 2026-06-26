package api

import (
	"errors"
	"net/http"

	"github.com/BAN1ce/skyTree/api/base"
	"github.com/BAN1ce/skyTree/internal/broker/acl"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/gin-gonic/gin"
)

func registerACLRoutes(v1 *gin.RouterGroup, comp *Component) {
	if v1 == nil {
		return
	}
	if comp == nil || comp.ACLManager == nil {
		logger.Logger.Warn().Msg("ACL admin API disabled: ACL manager is nil")
		return
	}
	if comp.ACLAdminUsername == "" || comp.ACLAdminPassword == "" {
		logger.Logger.Warn().Msg("ACL admin API disabled: basic auth username/password is empty")
		return
	}

	admin := v1.Group("/acl")
	admin.Use(gin.BasicAuth(gin.Accounts{comp.ACLAdminUsername: comp.ACLAdminPassword}))

	h := &aclHandler{mgr: comp.ACLManager}

	admin.GET("/ruleset", h.getRuleset)
	admin.PUT("/ruleset", h.putRuleset)
	admin.DELETE("/ruleset", h.deleteRuleset)

	admin.GET("/rule", h.getRule)
	admin.PUT("/rule", h.putRule)
	admin.DELETE("/rule", h.deleteRule)
}

type aclHandler struct {
	mgr *acl.Manager
}

func (h *aclHandler) getRuleset(c *gin.Context) {
	rs, ok, err := h.mgr.GetRuleset(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeNotFound(c, "ruleset not found")
		return
	}
	c.JSON(http.StatusOK, base.WithData(rs))
}

func (h *aclHandler) putRuleset(c *gin.Context) {
	if err := h.rejectWhenFileModeActive(c); err != nil {
		return
	}
	var rs acl.Ruleset
	if err := c.ShouldBindJSON(&rs); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := h.mgr.PutRuleset(c.Request.Context(), rs); err != nil {
		writeMgrErr(c, err)
		return
	}
	c.JSON(http.StatusOK, base.WithSuccess())
}

func (h *aclHandler) deleteRuleset(c *gin.Context) {
	if err := h.rejectWhenFileModeActive(c); err != nil {
		return
	}
	if err := h.mgr.DeleteRuleset(c.Request.Context()); err != nil {
		writeMgrErr(c, err)
		return
	}
	c.JSON(http.StatusOK, base.WithSuccess())
}

func (h *aclHandler) getRule(c *gin.Context) {
	id := parseIdentityQuery(c)
	if err := validateIdentityQuery(id); err != nil {
		writeBadRequest(c, err)
		return
	}
	r, ok, err := h.mgr.GetRule(c.Request.Context(), id)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeNotFound(c, "rule not found")
		return
	}
	c.JSON(http.StatusOK, base.WithData(r))
}

func (h *aclHandler) putRule(c *gin.Context) {
	if err := h.rejectWhenFileModeActive(c); err != nil {
		return
	}
	var r acl.Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := h.mgr.UpsertRule(c.Request.Context(), r); err != nil {
		writeMgrErr(c, err)
		return
	}
	c.JSON(http.StatusOK, base.WithSuccess())
}

func (h *aclHandler) deleteRule(c *gin.Context) {
	if err := h.rejectWhenFileModeActive(c); err != nil {
		return
	}
	id := parseIdentityQuery(c)
	if err := validateIdentityQuery(id); err != nil {
		writeBadRequest(c, err)
		return
	}
	removed, err := h.mgr.DeleteRule(c.Request.Context(), id)
	if err != nil {
		writeMgrErr(c, err)
		return
	}
	if !removed {
		writeNotFound(c, "rule not found")
		return
	}
	c.JSON(http.StatusOK, base.WithSuccess())
}

func (h *aclHandler) rejectWhenFileModeActive(c *gin.Context) error {
	if h.mgr != nil && h.mgr.FileModeActive() {
		writeErr(c, http.StatusConflict, acl.ErrFileModeActive)
		return acl.ErrFileModeActive
	}
	return nil
}

func parseIdentityQuery(c *gin.Context) acl.Identity {
	return acl.Identity{
		Username: c.Query("username"),
		ClientID: c.Query("client_id"),
	}
}

func validateIdentityQuery(id acl.Identity) error {
	if id.Username == "" && id.ClientID == "" {
		return errors.New("username and client_id are both empty")
	}
	return nil
}

func writeBadRequest(c *gin.Context, err error) {
	writeErr(c, http.StatusBadRequest, err)
}

func writeNotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, &base.Response{
		Code:    CodeNotFound,
		Msg:     msg,
		Success: false,
	})
}

func writeMgrErr(c *gin.Context, err error) {
	if errors.Is(err, acl.ErrFileModeActive) {
		writeErr(c, http.StatusConflict, err)
		return
	}
	if errors.Is(err, acl.ErrRulesetNotFound) {
		writeNotFound(c, err.Error())
		return
	}
	if errors.Is(err, acl.ErrInvalidIdentity) {
		writeBadRequest(c, err)
		return
	}
	writeErr(c, http.StatusInternalServerError, err)
}

func writeErr(c *gin.Context, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	c.JSON(status, base.WithError(err))
}
