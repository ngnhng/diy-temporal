package activities

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Order represents an order structure
type Order struct {
	ID      string  `json:"id"`
	UserID  string  `json:"user_id"`
	Items   []Item  `json:"items"`
	Total   float64 `json:"total"`
	Status  string  `json:"status"`
	Address Address `json:"address"`
}

type Item struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

// ValidateOrderActivity validates an order
type ValidateOrderActivity struct{}

func (a *ValidateOrderActivity) GetActivityType() string {
	return "validate_order"
}

func (a *ValidateOrderActivity) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// Simulate validation time
	time.Sleep(100 * time.Millisecond)

	// Extract order data
	orderData, ok := input["order"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid order data")
	}

	// Validate required fields
	userID, ok := orderData["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	items, ok := orderData["items"].([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("order must have at least one item")
	}

	// Validate each item
	totalAmount := 0.0
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid item format")
		}

		price, ok := itemMap["price"].(float64)
		if !ok || price <= 0 {
			return nil, fmt.Errorf("invalid item price")
		}

		quantity, ok := itemMap["quantity"].(float64)
		if !ok || quantity <= 0 {
			return nil, fmt.Errorf("invalid item quantity")
		}

		totalAmount += price * quantity
	}

	// Simulate random validation failure (5% chance)
	if rand.Float32() < 0.05 {
		return nil, fmt.Errorf("order validation failed: suspicious activity detected")
	}

	return map[string]any{
		"validation_status": "passed",
		"total_amount":      totalAmount,
		"validated_at":      time.Now().Format(time.RFC3339),
	}, nil
}

// ChargePaymentActivity processes payment for an order
type ChargePaymentActivity struct{}

func (a *ChargePaymentActivity) GetActivityType() string {
	return "charge_payment"
}

func (a *ChargePaymentActivity) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// Simulate payment processing time
	time.Sleep(200 * time.Millisecond)

	totalAmount, ok := input["total_amount"].(float64)
	if !ok {
		return nil, fmt.Errorf("total_amount is required")
	}

	orderData, ok := input["order"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("order data is required")
	}

	userID, ok := orderData["user_id"].(string)
	if !ok {
		return nil, fmt.Errorf("user_id is required")
	}

	// Simulate payment method validation
	paymentMethod, ok := input["payment_method"].(string)
	if !ok {
		paymentMethod = "credit_card"
	}

	// Simulate random payment failure (10% chance)
	if rand.Float32() < 0.10 {
		return nil, fmt.Errorf("payment failed: insufficient funds")
	}

	// Generate transaction ID
	transactionID := fmt.Sprintf("txn_%d_%s", time.Now().Unix(), userID)

	return map[string]any{
		"transaction_id": transactionID,
		"charged_amount": totalAmount,
		"payment_method": paymentMethod,
		"payment_status": "completed",
		"processed_at":   time.Now().Format(time.RFC3339),
	}, nil
}

// ShipOrderActivity handles order shipping
type ShipOrderActivity struct{}

func (a *ShipOrderActivity) GetActivityType() string {
	return "ship_order"
}

func (a *ShipOrderActivity) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// Simulate shipping processing time
	time.Sleep(150 * time.Millisecond)

	orderData, ok := input["order"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("order data is required")
	}

	_, ok = input["transaction_id"].(string)
	if !ok {
		return nil, fmt.Errorf("transaction_id is required")
	}

	// Extract shipping address
	address, ok := orderData["address"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("shipping address is required")
	}

	// Validate address
	street, _ := address["street"].(string)
	city, _ := address["city"].(string)
	state, _ := address["state"].(string)
	zipCode, _ := address["zip_code"].(string)

	if street == "" || city == "" || state == "" || zipCode == "" {
		return nil, fmt.Errorf("incomplete shipping address")
	}

	// Simulate random shipping failure (3% chance)
	if rand.Float32() < 0.03 {
		return nil, fmt.Errorf("shipping failed: address not serviceable")
	}

	// Generate tracking number
	trackingNumber := fmt.Sprintf("TRK%d%s", time.Now().Unix(), zipCode)

	// Calculate estimated delivery (3-5 business days)
	deliveryDays := rand.Intn(3) + 3
	estimatedDelivery := time.Now().AddDate(0, 0, deliveryDays)

	return map[string]any{
		"tracking_number":    trackingNumber,
		"shipping_status":    "shipped",
		"carrier":            "FastShip Express",
		"estimated_delivery": estimatedDelivery.Format("2006-01-02"),
		"shipping_address": map[string]any{
			"street":   street,
			"city":     city,
			"state":    state,
			"zip_code": zipCode,
		},
		"shipped_at": time.Now().Format(time.RFC3339),
	}, nil
}

// SendNotificationActivity sends notifications to customers
type SendNotificationActivity struct{}

func (a *SendNotificationActivity) GetActivityType() string {
	return "send_notification"
}

func (a *SendNotificationActivity) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// Simulate notification processing time
	time.Sleep(50 * time.Millisecond)

	orderData, ok := input["order"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("order data is required")
	}

	userID, ok := orderData["user_id"].(string)
	if !ok {
		return nil, fmt.Errorf("user_id is required")
	}

	notificationType, ok := input["notification_type"].(string)
	if !ok {
		notificationType = "order_shipped"
	}

	// Simulate different notification types
	var message string
	switch notificationType {
	case "order_confirmed":
		message = "Your order has been confirmed and payment processed successfully!"
	case "order_shipped":
		trackingNumber, _ := input["tracking_number"].(string)
		message = fmt.Sprintf("Your order has been shipped! Tracking number: %s", trackingNumber)
	default:
		message = "Order status update"
	}

	// Simulate notification channels
	channels := []string{"email", "sms"}
	notifications := make([]map[string]any, 0)

	for _, channel := range channels {
		// Simulate random notification failure (10% chance per channel)
		if rand.Float32() < 0.1 {
			continue
		}

		notifications = append(notifications, map[string]any{
			"channel":   channel,
			"recipient": userID,
			"message":   message,
			"sent_at":   time.Now().Format(time.RFC3339),
			"status":    "sent",
		})
	}

	if len(notifications) == 0 {
		return nil, fmt.Errorf("failed to send notifications on all channels")
	}

	return map[string]any{
		"notifications_sent": len(notifications),
		"notifications":      notifications,
		"notification_type":  notificationType,
	}, nil
}
