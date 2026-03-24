package handlers

import (
	"net/http"
	"strconv"

	"stellarbill-backend/internal/pagination"

	"github.com/gin-gonic/gin"
)

type Subscription struct {
	ID        string `json:"id"`
	PlanID    string `json:"plan_id"`
	Customer  string `json:"customer"`
	Status    string `json:"status"`
	Amount    string `json:"amount"`
	Interval  string `json:"interval"`
	NextBilling string `json:"next_billing,omitempty"`
}

// Ensure Subscription implements pagination.Item for in-memory processing
func (s Subscription) GetID() string        { return s.ID }
func (s Subscription) GetSortValue() string { return s.NextBilling }

func ListSubscriptions(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit value"})
		return
	}

	cursorStr := c.Query("cursor")
	cursor, err := pagination.Decode(cursorStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor format"})
		return
	}

	// TODO: load from DB, handle filtering
	// For now, we initialize an empty slice until DB is connected
	var mockDB []Subscription
	
	page, nextCursor, hasMore := pagination.PaginateSlice(mockDB, cursor, limit)

	c.JSON(http.StatusOK, gin.H{
		"data": page,
		"pagination": gin.H{
			"next_cursor": pagination.Encode(nextCursor),
			"has_more":    hasMore,
		},
	})
}

func GetSubscription(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription id required"})
		return
	}
	// TODO: load from DB by id
	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": "placeholder",
	})
}
