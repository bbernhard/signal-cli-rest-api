package client

type InvalidNameError struct {
	Description string
}

func (e *InvalidNameError) Error() string {
	return e.Description
}

type NotFoundError struct {
	Description string
}

func (e *NotFoundError) Error() string {
	return e.Description
}

type InternalError struct {
	Description string
}

func (e *InternalError) Error() string {
	return e.Description
}
