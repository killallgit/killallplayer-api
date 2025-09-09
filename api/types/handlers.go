package types

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler utility functions to reduce duplication across handlers

// ParseUintParam extracts and parses a URL parameter as uint
// Returns the parsed value and sends error response if parsing fails
func ParseUintParam(c *gin.Context, paramName string) (uint, bool) {
	paramStr := c.Param(paramName)
	value, err := strconv.ParseUint(paramStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid " + paramName,
		})
		return 0, false
	}
	return uint(value), true
}

// ParseInt64Param extracts and parses a URL parameter as int64
// Returns the parsed value and sends error response if parsing fails
func ParseInt64Param(c *gin.Context, paramName string) (int64, bool) {
	paramStr := c.Param(paramName)
	value, err := strconv.ParseInt(paramStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid " + paramName,
		})
		return 0, false
	}
	return value, true
}

// BindJSONOrError attempts to bind JSON request body to target struct
// Returns false and sends error response if binding fails
func BindJSONOrError(c *gin.Context, target interface{}) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Details: err.Error(),
		})
		return false
	}
	return true
}

// SendBadRequest sends a standardized bad request response
func SendBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Error: message})
}

// SendNotFound sends a standardized not found response
func SendNotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Error: message})
}

// SendInternalError sends a standardized internal server error response
func SendInternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: message})
}

// SendSuccess sends a standardized success response with data
func SendSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

// SendCreated sends a standardized created response with data
func SendCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}
