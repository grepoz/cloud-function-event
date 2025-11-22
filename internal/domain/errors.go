package domain

type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string {
	return e.Msg
}

func ErrValidation(msg string) error {
	return &ValidationError{Msg: msg}
}
