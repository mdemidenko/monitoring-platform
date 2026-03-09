package models

// Исходная структура Service
type Service struct {
	ID	interface{} `json:"id" bson:"id"`
 	Name     string      `json:"name" bson:"name"`
    Tenant   string      `json:"tenant" bson:"tenant"`
	DeprecatedDate string `json:"deprecated_date"`
	BusinessLine   string `json:"businessLine"`
	Clusters []string    `json:"clusters,omitempty" bson:"clusters,omitempty"`
}

// Структура результата фильтрации
type Result struct {
    ID     interface{} `json:"id" bson:"id"`
    Name   string      `json:"name" bson:"name"`
    Tenant string      `json:"tenant" bson:"tenant"`
	// Clusters []string    `json:"clusters,omitempty" bson:"clusters,omitempty"`
}

