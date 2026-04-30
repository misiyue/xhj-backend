package message

import (
	"testing"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestText_TextMessage(t *testing.T) {
	extra := `{"content":"Hello World","mentions":[]}`
	result := text(entity.ChatMsgTypeText, extra)
	assert.Equal(t, "Hello World", result)
}

func TestText_TextMessageTruncation(t *testing.T) {
	// Generate a string longer than 200 characters
	longContent := ""
	for i := 0; i < 210; i++ {
		longContent += "a"
	}
	extra := `{"content":"` + longContent + `"}`
	result := text(entity.ChatMsgTypeText, extra)
	assert.LessOrEqual(t, len(result), 200)
}

func TestText_ImageMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeImage, "")
	assert.Equal(t, "[图片消息]", result)
}

func TestText_AudioMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeAudio, "")
	assert.Equal(t, "[语音消息]", result)
}

func TestText_VideoMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeVideo, "")
	assert.Equal(t, "[视频消息]", result)
}

func TestText_FileMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeFile, "")
	assert.Equal(t, "[文件消息]", result)
}

func TestText_UnknownMessage(t *testing.T) {
	result := text(9999, "")
	assert.Equal(t, "未知消息", result)
}

func TestText_InvalidJSON(t *testing.T) {
	result := text(entity.ChatMsgTypeText, "invalid json")
	assert.Equal(t, "", result)
}

func TestText_RTCCallMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeRTCCall, "")
	assert.Equal(t, "[通话记录]", result)
}

func TestText_RedEnvelopeMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeRedEnvelope, "")
	assert.Equal(t, "[红包]", result)
}

func TestText_TransferMessage(t *testing.T) {
	result := text(entity.ChatMsgTypeTransfer, "")
	assert.Equal(t, "[转账]", result)
}
