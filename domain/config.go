package domain

type ConfigRepo[T any] interface {
	Set(value T) bool
	Get() T
	CAS(old, new T) bool
}

type ConfigService[T any] interface {
	Set(value T) bool
	Get() T
	CAS(old, new T) bool
}

type TicketPlusConfig struct {
	ReserveDuration int `json:"reserve_duration"`
}
