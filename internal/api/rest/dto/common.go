package dto

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type PaginatedResponse struct {
	Data   interface{} `json:"data"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type HealthResponse struct {
	Status string `json:"status"`
}
