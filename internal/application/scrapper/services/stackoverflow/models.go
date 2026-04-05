package stackoverflow

type QuestionResponse struct {
	Items []Question `json:"items"`
}

type Question struct {
	QuestionID       int64  `json:"question_id"`
	Title            string `json:"title"`
	Link             string `json:"link"`
	LastActivityDate int64  `json:"last_activity_date"`
}

type AnswerResponse struct {
	Items []AnswerOrComment `json:"items"`
}

type CommentResponse struct {
	Items []AnswerOrComment `json:"items"`
}

type AnswerOrComment struct {
	Body         string `json:"body"`
	CreationDate int64  `json:"creation_date"`
	Owner        Owner  `json:"owner"`
	Link         string `json:"link"`
}

type Owner struct {
	DisplayName string `json:"display_name"`
}
