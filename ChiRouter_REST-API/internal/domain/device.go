package domain

import "time"

type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IPAddress string    `json:"ip_address"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}
