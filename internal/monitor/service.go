package monitor


import (
	"github.com/mdemidenko/monitoring-platform/internal/models"
	"github.com/mdemidenko/monitoring-platform/internal/repository"
)


type Service interface {
	FilterServices() ([]models.Result, error)
}

const (
	TargetDeprecatedDate = "0001-01-01T00:00:00Z"
	TargetBusinessLine   = "Управление разработки решений для бизнеса и Центр оптимизации процессов поставки"
)

type service struct {
	repo repository.Repository
}

func New(repo repository.Repository) Service {
	return &service{repo: repo}
}

func (s *service) FilterServices() ([]models.Result, error) {
	services, err := s.repo.GetServices()
	if err != nil {
		return nil, err
	}

	var results []models.Result
	for _, svc := range services {
		if svc.DeprecatedDate == TargetDeprecatedDate && svc.BusinessLine == TargetBusinessLine {
			results = append(results, models.Result{
				ID:     svc.ID,
				Name:   svc.Name,
				Tenant: svc.Tenant,
			})
		}
	}

	return results, nil
}