package main

type TransportError struct {
	Cause     error
	Temporary bool
}

func NewTransportError(cause error, temporary bool) TransportError {
	return TransportError{
		Cause:     cause,
		Temporary: temporary,
	}
}

func (e TransportError) Error() string {
	return e.Cause.Error()
}
