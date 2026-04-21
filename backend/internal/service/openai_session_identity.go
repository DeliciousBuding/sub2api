package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type openAISessionIdentity struct {
	SessionSeed       string
	SessionHash       string
	LegacySessionHash string
	SessionSource     string
	PromptCacheKey    string
	PromptCacheSource string
}

func (s *OpenAIGatewayService) ResolveSessionIdentity(c *gin.Context, body []byte) openAISessionIdentity {
	identity := openAISessionIdentity{
		SessionSource:     "none",
		PromptCacheSource: "none",
	}

	metadataUserID := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())
	if parsed := ParseMetadataUserID(metadataUserID); parsed != nil && strings.TrimSpace(parsed.SessionID) != "" {
		sessionID := strings.TrimSpace(parsed.SessionID)
		identity.SessionSeed = sessionID
		identity.PromptCacheKey = sessionID
		identity.SessionSource = "metadata_user_id_session"
		identity.PromptCacheSource = "metadata_user_id_session"
		return identity.withHashes()
	}

	if sessionID := firstNonEmptyHeader(c, "X-Session-ID", "session_id", "Session_id"); sessionID != "" {
		identity.SessionSeed = sessionID
		identity.PromptCacheKey = sessionID
		identity.SessionSource = "header_session_id"
		identity.PromptCacheSource = "header_session_id"
		return identity.withHashes()
	}

	if conversationID := firstNonEmptyHeader(c, "conversation_id", "Conversation_id"); conversationID != "" {
		identity.SessionSeed = conversationID
		identity.PromptCacheKey = conversationID
		identity.SessionSource = "header_conversation_id"
		identity.PromptCacheSource = "header_conversation_id"
		return identity.withHashes()
	}

	if metadataUserID != "" {
		identity.SessionSeed = "metadata_user_id:" + metadataUserID
		identity.PromptCacheKey = GenerateSessionUUID(identity.SessionSeed)
		identity.SessionSource = "metadata_user_id"
		identity.PromptCacheSource = "metadata_user_id"
		return identity.withHashes()
	}

	if conversationID := strings.TrimSpace(gjson.GetBytes(body, "conversation_id").String()); conversationID != "" {
		identity.SessionSeed = conversationID
		identity.PromptCacheKey = conversationID
		identity.SessionSource = "body_conversation_id"
		identity.PromptCacheSource = "body_conversation_id"
		return identity.withHashes()
	}

	if promptCacheKey := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); promptCacheKey != "" {
		identity.SessionSeed = promptCacheKey
		identity.PromptCacheKey = promptCacheKey
		identity.SessionSource = "prompt_cache_key"
		identity.PromptCacheSource = "prompt_cache_key"
		return identity.withHashes()
	}

	if contentSeed := deriveOpenAIContentSessionSeed(body); contentSeed != "" {
		identity.SessionSeed = contentSeed
		identity.SessionSource = "content_fallback"
		return identity.withHashes()
	}

	return identity
}

func (i openAISessionIdentity) withHashes() openAISessionIdentity {
	if strings.TrimSpace(i.SessionSeed) == "" {
		return i
	}
	i.SessionHash, i.LegacySessionHash = deriveOpenAISessionHashes(i.SessionSeed)
	return i
}

func firstNonEmptyHeader(c *gin.Context, keys ...string) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return firstNonEmptyHTTPHeader(c.Request.Header, keys...)
}

func firstNonEmptyHTTPHeader(headers http.Header, keys ...string) string {
	if headers == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			return value
		}
	}
	return ""
}
