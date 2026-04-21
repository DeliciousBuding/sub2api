package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveOpenAIWSSessionHeaders_SupportsExtendedHeaderForms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/openai/v1/responses/ws", nil)
	c.Request.Header = http.Header{}
	c.Request.Header.Set("X-Session-ID", "x-session")
	c.Request.Header.Set("Conversation_id", "conv-header")

	resolution := resolveOpenAIWSSessionHeaders(c, "")
	require.Equal(t, "x-session", resolution.SessionID)
	require.Equal(t, "conv-header", resolution.ConversationID)
	require.Equal(t, "header_session_id", resolution.SessionSource)
	require.Equal(t, "header_conversation_id", resolution.ConversationSource)
}

func TestResolveOpenAIWSSessionHeaders_PromptCacheKeyFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/openai/v1/responses/ws", nil)

	resolution := resolveOpenAIWSSessionHeaders(c, "prompt-cache-key")
	require.Equal(t, "prompt-cache-key", resolution.SessionID)
	require.Equal(t, "prompt_cache_key", resolution.SessionSource)
	require.Empty(t, resolution.ConversationID)
	require.Equal(t, "none", resolution.ConversationSource)
}
