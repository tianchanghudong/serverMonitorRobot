package models

//钉钉数据
type DingDingData struct {
	Msg               map[string]string `json:"text"`
	CreateAt          int               `json:"createAt"`
	SenderId          string            `json:"senderId"`
	SenderNick        string            `json:"senderNick"`
	SenderCorpId      string            `json:"senderCorpId"`
	SenderStaffId     string            `json:"senderStaffId"`
	ChatbotUserId     string            `json:"chatbotUserId"`
	SessionWebhook    string            `json:"sessionWebhook"`
	ConversationTitle string            `json:"conversationTitle"`
}
