package textblock

import "fmt"

type Error struct {
	Line int
}

type ErrorUnexpectedStart Error
type ErrorUnexpectedEnd Error
type ErrorMissingEnd Error

func (e *ErrorUnexpectedStart) Error() string {
	return fmt.Sprintf("error: unexpected start string at line %d", e.Line)
}

func IsErrorUnexpectedStart(err error) bool {
	_, ok := err.(*ErrorUnexpectedStart)
	return ok
}

func (e *ErrorUnexpectedEnd) Error() string {
	return fmt.Sprintf("error: unexpected end string at line %d", e.Line)
}

func IsErrorUnexpectedEnd(err error) bool {
	_, ok := err.(*ErrorUnexpectedEnd)
	return ok
}

func (e *ErrorMissingEnd) Error() string {
	return fmt.Sprintf("error: missing end string for block starting at line %d", e.Line)
}

func IsErrorMissingEnd(err error) bool {
	_, ok := err.(*ErrorMissingEnd)
	return ok
}
