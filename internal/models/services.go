package models

// Исходная структура Service
type Service struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Tenant         string `json:"tenant"`
	DeprecatedDate string `json:"deprecated_date"`
	BusinessLine   string `json:"businessLine"`
}

// Структура результата фильтрации
type Result struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Tenant string `json:"tenant"`
}