package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) rejectSensitiveWordForAnthropic(c *gin.Context, body []byte) bool {
	if !service.CheckSensitiveWordPolicy(c.Request.Context(), h.settingService, body) {
		return false
	}
	h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", service.SensitiveWordBlockedMessage)
	return true
}

func (h *GatewayHandler) rejectSensitiveWordForOpenAIResponses(c *gin.Context, body []byte) bool {
	if !service.CheckSensitiveWordPolicy(c.Request.Context(), h.settingService, body) {
		return false
	}
	h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", service.SensitiveWordBlockedMessage)
	return true
}

func (h *GatewayHandler) rejectSensitiveWordForOpenAIChat(c *gin.Context, body []byte) bool {
	if !service.CheckSensitiveWordPolicy(c.Request.Context(), h.settingService, body) {
		return false
	}
	h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", service.SensitiveWordBlockedMessage)
	return true
}

func (h *GatewayHandler) rejectSensitiveWordForGemini(c *gin.Context, body []byte) bool {
	if !service.CheckSensitiveWordPolicy(c.Request.Context(), h.settingService, body) {
		return false
	}
	googleError(c, http.StatusBadRequest, service.SensitiveWordBlockedMessage)
	return true
}

func (h *OpenAIGatewayHandler) rejectSensitiveWordForOpenAI(c *gin.Context, body []byte) bool {
	if h == nil || h.gatewayService == nil || !h.gatewayService.SensitiveWordFilterMatched(c.Request.Context(), body) {
		return false
	}
	h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", service.SensitiveWordBlockedMessage)
	return true
}

func (h *OpenAIGatewayHandler) rejectSensitiveWordForAnthropic(c *gin.Context, body []byte) bool {
	if h == nil || h.gatewayService == nil || !h.gatewayService.SensitiveWordFilterMatched(c.Request.Context(), body) {
		return false
	}
	h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", service.SensitiveWordBlockedMessage)
	return true
}
