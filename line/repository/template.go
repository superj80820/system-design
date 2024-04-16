package repository

import (
	"fmt"

	"github.com/superj80820/system-design/domain"
)

type lineTemplateRepo struct {
	templates map[string]string
}

func CreateLineTemplate(templates map[string]string) domain.LineTemplateRepo {
	return &lineTemplateRepo{
		templates: templates,
	}
}

func (l *lineTemplateRepo) ApplyTemplate(name string, args ...any) (string, error) {
	if val, ok := l.templates[name]; ok {
		return fmt.Sprintf(val, args...), nil
	}
	return "", domain.ErrNoData
}
