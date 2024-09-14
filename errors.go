package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type errorWithMessage struct {
	code    int
	message string
}

func newErrorWithMessage(code int, format string, args ...any) errorWithMessage {
	return errorWithMessage{
		code:    code,
		message: fmt.Sprintf(format, args...),
	}
}

func (e errorWithMessage) Error() string {
	return e.message
}

func handleWebcalErr(c *gin.Context, err error) {
	var msgErr errorWithMessage
	if errors.As(err, &msgErr) {
		c.String(msgErr.code, msgErr.message)
		return
	}

	log(c).Error(err)

	handleWebcalErr(c, newErrorWithMessage(
		http.StatusInternalServerError,
		"%s", http.StatusText(http.StatusInternalServerError),
	))
}

func handleHTMXError(c *gin.Context, calendar Calendar, err error) {
	var msgErr errorWithMessage
	if errors.As(err, &msgErr) {
		calendar.Error = msgErr.message
		c.HTML(http.StatusOK, "calendar", calendar)
		return
	}

	log(c).Error(err)

	handleHTMXError(c, calendar, newErrorWithMessage(
		http.StatusInternalServerError,
		"%s", http.StatusText(http.StatusInternalServerError),
	))
}
