package settings

import "context"

type Service struct {
	repo *Repo
}

func (s *Service) Publicas(ctx context.Context) ([]Setting, error) {
	return s.repo.listar(ctx, true)
}

func (s *Service) Todas(ctx context.Context) ([]Setting, error) {
	return s.repo.listar(ctx, false)
}
