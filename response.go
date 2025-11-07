package pkg

type Response[T any] struct {
	Status  bool   `json:"status"`
	Code    int    `json:"code"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

func (t Response[T]) Success(data T, message string) Response[T] {
	return Response[T]{
		Status:  true,
		Code:    200,
		Data:    data,
		Message: message,
	}
}

func (t Response[T]) Error(message string) Response[T] {
	return Response[T]{
		Status:  true,
		Code:    400,
		Message: message,
	}
}
