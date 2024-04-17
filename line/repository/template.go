package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type lineTemplateRepo struct {
	templates map[string]string
}

func CreateLineTemplate(templates map[string]string) (domain.LineTemplateRepo, error) {
	l := &lineTemplateRepo{
		templates: make(map[string]string),
	}

	for name, template := range templates {
		compactTemplate := new(bytes.Buffer)
		if err := json.Compact(compactTemplate, []byte(template)); err != nil {
			return nil, errors.Wrap(err, "compact failed")
		}
		l.templates[name] = strings.ReplaceAll(compactTemplate.String(), `"^$"`, "%s")
	}
	return l, nil
}

func (l *lineTemplateRepo) ApplyTemplate(name string, args ...any) (string, error) {
	for idx, arg := range args {
		switch v := arg.(type) {
		case string:
			args[idx] = fmt.Sprintf(`"%s"`, v)
		case []string:
			args[idx] = "[" + strings.Join(v, ",") + "]"
		}
	}
	if val, ok := l.templates[name]; ok {
		return fmt.Sprintf(val, args...), nil
	}
	return "", domain.ErrNoData
}

func (l *lineTemplateRepo) ApplyText(text string) string {
	return fmt.Sprintf(`{"type":"text","text":"%s"}`, text)
}
