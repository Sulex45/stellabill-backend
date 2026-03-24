package handlers

import (
	"net/http"
	"strconv"

	"stellarbill-backend/internal/pagination"

	"github.com/gin-gonic/gin"
)

type Plan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Amount      string `json:"amount"` // Used as secondary sort value if needed
	Currency    string `json:"currency"`
	Interval    string `json:"interval"`
	Description string `json:"description,omitempty"`
}

// Ensure Plan implements pagination.Item for in-memory processing
func (p Plan) GetID() string        { return p.ID }
func (p Plan) GetSortValue() string { return p.Amount }

func ListPlans(c *gin.Context) {
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

	// TODO: load from DB using limit and cursor
	// For now, we initialize an empty slice until DB is connected
	var mockDB []Plan
	
	page, nextCursor, hasMore := pagination.PaginateSlice(mockDB, cursor, limit)

	c.JSON(http.StatusOK, gin.H{
		"data": page,
		"pagination": gin.H{
			"next_cursor": pagination.Encode(nextCursor),
			"has_more":    hasMore,
		},
	})
}
