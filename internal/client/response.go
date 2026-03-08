package client

import "github.com/gin-gonic/gin"

func errorResponse(c *gin.Context, status int, message string) {
	reqID, _ := c.Get("requestID")
	c.JSON(status, gin.H{
		"error":     message,
		"requestId": reqID,
	})
}
